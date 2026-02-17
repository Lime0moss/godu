package scanner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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
	inodeMap := make(map[inodeKey]bool)

	// Progress tracking
	var filesScanned, dirsScanned, bytesFound, errCount atomic.Int64
	startTime := time.Now()

	// Exclude set for fast lookup
	excludeSet := make(map[string]bool, len(opts.ExcludePatterns))
	for _, p := range opts.ExcludePatterns {
		excludeSet[p] = true
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
	updateSizeRecursive(root)

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
	inodeMap map[inodeKey]bool,
	inodeMu *sync.Mutex,
	excludeSet map[string]bool,
	visitedDirs *sync.Map,
) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		errCount.Add(1)
		return
	}

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

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		name := entry.Name()

		// Skip hidden files if not showing them
		if !opts.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// Skip excluded patterns
		if excludeSet[name] {
			continue
		}

		fullPath := filepath.Join(dirPath, name)

		if entry.IsDir() {
			scanPath := fullPath
			if resolvedPath, err := filepath.EvalSymlinks(fullPath); err == nil {
				scanPath = resolvedPath
			}

			childDir := &model.DirNode{
				FileNode: model.FileNode{
					Name:   name,
					Parent: parent,
				},
			}

			if info, err := entry.Info(); err == nil {
				childDir.Mtime = info.ModTime()
			}

			parent.AddChild(childDir)

			// Already visited via another path (e.g. followed symlink): keep node,
			// but skip recursion so size is not double-counted.
			if _, loaded := visitedDirs.LoadOrStore(scanPath, true); loaded {
				continue
			}

			spawnScan(scanPath, childDir)
		} else if entry.Type()&os.ModeSymlink != 0 && opts.FollowSymlinks {
			// Resolve symlink — if it points to a directory, recurse into it
			resolvedPath, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				errCount.Add(1)
				continue
			}
			targetInfo, err := os.Stat(resolvedPath)
			if err != nil {
				errCount.Add(1)
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

			var inode uint64
			var diskUsage int64

			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				inode = stat.Ino
				diskUsage = int64(stat.Blocks) * 512
			} else {
				diskUsage = info.Size()
			}

			fileNode := &model.FileNode{
				Name:   name,
				Size:   info.Size(),
				Usage:  diskUsage,
				Mtime:  info.ModTime(),
				Inode:  inode,
				Flag:   model.FlagSymlink,
				Parent: parent,
			}
			parent.AddChild(fileNode)
			filesScanned.Add(1)
			bytesFound.Add(info.Size())
		} else {
			info, err := entry.Info()
			if err != nil {
				errCount.Add(1)
				continue
			}

			var flag model.NodeFlag
			var inode uint64
			var diskUsage int64

			if entry.Type()&os.ModeSymlink != 0 {
				flag = model.FlagSymlink
			}

			// Get inode and disk usage from syscall stat
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				inode = stat.Ino
				diskUsage = int64(stat.Blocks) * 512 // blocks are 512-byte units

				// Hardlink detection
				if stat.Nlink > 1 {
					inodeMu.Lock()
					if inodeMap[inodeKey{dev: uint64(stat.Dev), ino: stat.Ino}] {
						flag |= model.FlagHardlink
						inodeMu.Unlock()
						// Still add the node but don't count size twice
						fileNode := &model.FileNode{
							Name:   name,
							Size:   0,
							Usage:  0,
							Mtime:  info.ModTime(),
							Inode:  inode,
							Flag:   flag,
							Parent: parent,
						}
						parent.AddChild(fileNode)
						filesScanned.Add(1)
						continue
					}
					inodeMap[inodeKey{dev: uint64(stat.Dev), ino: stat.Ino}] = true // uses both dev+ino to avoid cross-filesystem inode collisions
					inodeMu.Unlock()
				}
			} else {
				diskUsage = info.Size()
			}

			fileNode := &model.FileNode{
				Name:   name,
				Size:   info.Size(),
				Usage:  diskUsage,
				Mtime:  info.ModTime(),
				Inode:  inode,
				Flag:   flag,
				Parent: parent,
			}

			parent.AddChild(fileNode)
			filesScanned.Add(1)
			bytesFound.Add(info.Size())
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

// updateSizeRecursive performs a bottom-up size calculation.
// Children are updated before parents, ensuring correct totals.
func updateSizeRecursive(dir *model.DirNode) {
	for _, child := range dir.Children {
		if cd, ok := child.(*model.DirNode); ok {
			updateSizeRecursive(cd)
		}
	}
	dir.UpdateSize()
}
