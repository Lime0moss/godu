package scanner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sadopc/godu/internal/model"
)

// inodeKey uniquely identifies a file across filesystems using both device and
// inode number. Using inode alone can cause false dedup on cross-filesystem scans.
type inodeKey struct {
	dev uint64
	ino uint64
}

// ParallelScanner implements Scanner with goroutine-per-directory parallelism.
type ParallelScanner struct{}

// NewParallelScanner creates a new parallel scanner.
func NewParallelScanner() *ParallelScanner {
	return &ParallelScanner{}
}

func (s *ParallelScanner) Scan(ctx context.Context, path string, opts ScanOptions, progress chan<- Progress) (*model.DirNode, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Use Stat (not Lstat) so symlinked directories like /tmp -> /private/tmp work
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "scan", Path: absPath, Err: os.ErrInvalid}
	}
	// Resolve symlinks for the root path
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		absPath = resolved
	}

	// Optionally disable GC during scan
	var oldGC int
	if opts.DisableGC {
		oldGC = debug.SetGCPercent(-1)
		defer debug.SetGCPercent(oldGC)
	}

	// Determine concurrency
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.GOMAXPROCS(0) * 3
	}
	sem := make(chan struct{}, concurrency)

	// Hardlink tracking (keyed by device+inode to avoid cross-filesystem collisions)
	var inodeMu sync.Mutex
	inodeMap := make(map[inodeKey]struct{})

	// Progress tracking
	var filesScanned, dirsScanned, bytesFound, errCount atomic.Int64
	startTime := time.Now()

	// Exclude set for fast lookup
	excludeSet := make(map[string]struct{}, len(opts.ExcludePatterns))
	for _, p := range opts.ExcludePatterns {
		excludeSet[p] = struct{}{}
	}

	// Create root node
	root := &model.DirNode{
		FileNode: model.FileNode{
			Name:  absPath,
			Mtime: info.ModTime(),
		},
	}

	// Progress reporter goroutine
	var progressWg sync.WaitGroup
	progressDone := make(chan struct{})
	if progress != nil {
		progressWg.Add(1)
		go func() {
			defer progressWg.Done()
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					elapsed := time.Since(startTime)
					select {
					case progress <- Progress{
						FilesScanned: filesScanned.Load(),
						DirsScanned:  dirsScanned.Load(),
						BytesFound:   bytesFound.Load(),
						Errors:       errCount.Load(),
						StartTime:    startTime,
						Duration:     elapsed,
					}:
					default:
						// Drop if channel full
					}
				case <-progressDone:
					return
				}
			}
		}()
	}

	// Track visited directories by canonical path to avoid cycles and duplicates.
	var visitedDirs sync.Map
	visitedDirs.Store(absPath, true)

	// Recursive parallel scan
	var wg sync.WaitGroup
	s.scanDir(ctx, absPath, absPath, root, opts, sem, &wg, &filesScanned, &dirsScanned, &bytesFound, &errCount, inodeMap, &inodeMu, excludeSet, &visitedDirs)
	wg.Wait()

	// Bottom-up size calculation after all goroutines complete
	if err := ctx.Err(); err != nil {
		return root, err
	}
	root.UpdateSizeRecursiveContext(ctx)

	// Send final progress
	if progress != nil {
		close(progressDone)
		progressWg.Wait()
		elapsed := time.Since(startTime)
		select {
		case progress <- Progress{
			FilesScanned: filesScanned.Load(),
			DirsScanned:  dirsScanned.Load(),
			BytesFound:   bytesFound.Load(),
			Errors:       errCount.Load(),
			Done:         true,
			StartTime:    startTime,
			Duration:     elapsed,
		}:
		default:
		}
	}

	if err := ctx.Err(); err != nil {
		return root, err
	}
	return root, nil
}

func (s *ParallelScanner) scanDir(
	ctx context.Context,
	scanRoot string,
	dirPath string,
	parent *model.DirNode,
	opts ScanOptions,
	sem chan struct{},
	wg *sync.WaitGroup,
	filesScanned, dirsScanned, bytesFound, errCount *atomic.Int64,
	inodeMap map[inodeKey]struct{},
	inodeMu *sync.Mutex,
	excludeSet map[string]struct{},
	visitedDirs *sync.Map,
) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	dir, err := os.Open(dirPath)
	if err != nil {
		parent.Flag |= model.FlagError
		errCount.Add(1)
		return
	}
	defer dir.Close()

	dirsScanned.Add(1)

	// Run subdirectory scans with bounded goroutines.
	// If all workers are busy, scan synchronously in the current goroutine
	// instead of spawning blocked goroutines.
	spawnScan := func(path string, dir *model.DirNode) {
		select {
		case sem <- struct{}{}:
			wg.Add(1)
			go func(p string, d *model.DirNode) {
				defer wg.Done()
				defer func() { <-sem }()
				s.scanDir(ctx, scanRoot, p, d, opts, sem, wg, filesScanned, dirsScanned, bytesFound, errCount, inodeMap, inodeMu, excludeSet, visitedDirs)
			}(path, dir)
		default:
			s.scanDir(ctx, scanRoot, path, dir, opts, sem, wg, filesScanned, dirsScanned, bytesFound, errCount, inodeMap, inodeMu, excludeSet, visitedDirs)
		}
	}

	for {
		entries, readErr := dir.ReadDir(256)

		for _, entry := range entries {
			select {
			case <-ctx.Done():
				return
			default:
			}

			name := entry.Name()

			// Skip excluded patterns
			if _, excluded := excludeSet[name]; excluded {
				continue
			}

			// Skip hidden files/dirs when ShowHidden is false
			if !opts.ShowHidden && len(name) > 0 && name[0] == '.' {
				continue
			}

			fullPath := filepath.Join(dirPath, name)
			info, err := entry.Info()
			if err != nil {
				errCount.Add(1)
				continue
			}

			mode := entry.Type()
			infoMode := info.Mode()
			// Some filesystems may return unknown dirent types; use FileInfo mode as fallback.
			if mode == 0 {
				mode = infoMode.Type()
			}
			if infoMode.IsDir() {
				mode |= os.ModeDir
			}
			if infoMode&os.ModeSymlink != 0 {
				mode |= os.ModeSymlink
			}

			// Skip special files (devices, sockets, pipes, irregular).
			// Check both dirent type and FileInfo mode for DT_UNKNOWN filesystems.
			if isSpecialMode(mode) || isSpecialMode(infoMode) {
				continue
			}

			if mode.IsDir() {
				scanPath := fullPath
				if opts.FollowSymlinks {
					if resolvedPath, err := filepath.EvalSymlinks(fullPath); err == nil {
						scanPath = resolvedPath
					}
				}

				childDir := &model.DirNode{
					FileNode: model.FileNode{
						Name:   name,
						Parent: parent,
					},
				}
				childDir.Mtime = info.ModTime()

				parent.AddChild(childDir)

				// Already visited via another path (e.g. followed symlink): keep node,
				// but skip recursion so size is not double-counted.
				if _, loaded := visitedDirs.LoadOrStore(scanPath, true); loaded {
					continue
				}

				spawnScan(scanPath, childDir)
			} else if mode&os.ModeSymlink != 0 && opts.FollowSymlinks {
				// Resolve symlink — if it points to a directory, recurse into it
				resolvedPath, err := filepath.EvalSymlinks(fullPath)
				if err != nil {
					errCount.Add(1)
					parent.AddChild(model.NewBrokenSymlinkNode(name, parent))
					filesScanned.Add(1)
					continue
				}
				targetInfo, err := os.Stat(resolvedPath)
				if err != nil {
					errCount.Add(1)
					parent.AddChild(model.NewBrokenSymlinkNode(name, parent))
					filesScanned.Add(1)
					continue
				}
				if isSpecialMode(targetInfo.Mode()) {
					continue
				}
				if targetInfo.IsDir() {
					childDir := &model.DirNode{
						FileNode: model.FileNode{
							Name:   name,
							Mtime:  targetInfo.ModTime(),
							Flag:   model.FlagSymlink,
							Parent: parent,
						},
					}
					parent.AddChild(childDir)

					// Avoid duplicate traversal for symlinks pointing inside the scan root.
					// The canonical in-tree directory will be scanned via normal traversal.
					if isWithin(scanRoot, resolvedPath) {
						continue
					}

					// If target was already scanned, don't recurse again.
					if _, loaded := visitedDirs.LoadOrStore(resolvedPath, true); loaded {
						continue
					}

					spawnScan(resolvedPath, childDir)
					continue
				}
				// Symlink to file — fall through to file handling below
				info := targetInfo

				flag := model.FlagSymlink
				si := getStatInfo(info)

				if si.ok {
					// Dedup: symlink target may alias a regular file (even with Nlink==1)
					inodeMu.Lock()
					ik := inodeKey{dev: si.dev, ino: si.inode}
					if _, seen := inodeMap[ik]; seen {
						flag |= model.FlagHardlink
						inodeMu.Unlock()
						fileNode := &model.FileNode{
							Name:   name,
							Size:   0,
							Usage:  0,
							Mtime:  info.ModTime(),
							Inode:  si.inode,
							Flag:   flag,
							Parent: parent,
						}
						parent.AddChild(fileNode)
						filesScanned.Add(1)
						continue
					}
					inodeMap[ik] = struct{}{}
					inodeMu.Unlock()
				}

				fileNode := &model.FileNode{
					Name:   name,
					Size:   info.Size(),
					Usage:  si.diskUsage,
					Mtime:  info.ModTime(),
					Inode:  si.inode,
					Flag:   flag,
					Parent: parent,
				}
				parent.AddChild(fileNode)
				filesScanned.Add(1)
				bytesFound.Add(info.Size())
			} else {
				var flag model.NodeFlag
				if mode&os.ModeSymlink != 0 {
					flag = model.FlagSymlink
				}

				si := getStatInfo(info)

				// Hardlink detection (also dedup when following symlinks to avoid double-counting)
				if si.ok && (si.nlink > 1 || opts.FollowSymlinks) {
					inodeMu.Lock()
					ik := inodeKey{dev: si.dev, ino: si.inode}
					if _, seen := inodeMap[ik]; seen {
						flag |= model.FlagHardlink
						inodeMu.Unlock()
						// Still add the node but don't count size twice
						fileNode := &model.FileNode{
							Name:   name,
							Size:   0,
							Usage:  0,
							Mtime:  info.ModTime(),
							Inode:  si.inode,
							Flag:   flag,
							Parent: parent,
						}
						parent.AddChild(fileNode)
						filesScanned.Add(1)
						continue
					}
					inodeMap[ik] = struct{}{}
					inodeMu.Unlock()
				}

				fileNode := &model.FileNode{
					Name:   name,
					Size:   info.Size(),
					Usage:  si.diskUsage,
					Mtime:  info.ModTime(),
					Inode:  si.inode,
					Flag:   flag,
					Parent: parent,
				}

				parent.AddChild(fileNode)
				filesScanned.Add(1)
				bytesFound.Add(info.Size())
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			parent.Flag |= model.FlagError
			errCount.Add(1)
			return
		}
	}
}

func isWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isSpecialMode(mode os.FileMode) bool {
	return mode&(os.ModeDevice|os.ModeCharDevice|os.ModeSocket|os.ModeNamedPipe|os.ModeIrregular) != 0
}
