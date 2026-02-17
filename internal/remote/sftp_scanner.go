package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	pathpkg "path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
	"github.com/sadopc/godu/internal/model"
	"github.com/sadopc/godu/internal/scanner"
	"golang.org/x/crypto/ssh"
)

const defaultRemotePath = "."

// Config configures a remote SFTP scan.
type Config struct {
	Target    string
	Port      int
	BatchMode bool
}

// SFTPScanner scans a remote filesystem over the SFTP subsystem.
type SFTPScanner struct {
	cfg  Config
	dial func(context.Context, Config) (sftpClient, io.Closer, error)
}

type sftpClient interface {
	ReadDir(string) ([]os.FileInfo, error)
	Stat(string) (os.FileInfo, error)
	ReadLink(string) (string, error)
	RealPath(string) (string, error)
}

// NewSFTPScanner creates a new remote scanner.
func NewSFTPScanner(cfg Config) *SFTPScanner {
	return &SFTPScanner{cfg: cfg, dial: dialSFTP}
}

// Scan scans a remote path over SFTP and returns the root directory tree.
func (s *SFTPScanner) Scan(ctx context.Context, remotePath string, opts scanner.ScanOptions, progress chan<- scanner.Progress) (*model.DirNode, error) {
	if s == nil {
		return nil, fmt.Errorf("remote scanner is nil")
	}
	if s.dial == nil {
		s.dial = dialSFTP
	}

	client, closer, err := s.dial(ctx, s.cfg)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return s.scanWithClient(ctx, client, remotePath, opts, progress)
}

func (s *SFTPScanner) scanWithClient(ctx context.Context, client sftpClient, remotePath string, opts scanner.ScanOptions, progress chan<- scanner.Progress) (*model.DirNode, error) {
	if strings.TrimSpace(remotePath) == "" {
		remotePath = defaultRemotePath
	}

	rootPath := cleanRemotePath(remotePath)
	if resolved, err := client.RealPath(rootPath); err == nil {
		rootPath = cleanRemotePath(resolved)
	}

	info, err := client.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat remote path %q: %w", rootPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", rootPath)
	}

	root := &model.DirNode{
		FileNode: model.FileNode{
			Name:  rootPath,
			Mtime: info.ModTime(),
		},
	}

	excludeSet := make(map[string]bool, len(opts.ExcludePatterns))
	for _, p := range opts.ExcludePatterns {
		excludeSet[p] = true
	}

	var filesScanned, dirsScanned, bytesFound, errCount atomic.Int64
	startTime := time.Now()

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
					case progress <- scanner.Progress{
						FilesScanned: filesScanned.Load(),
						DirsScanned:  dirsScanned.Load(),
						BytesFound:   bytesFound.Load(),
						Errors:       errCount.Load(),
						StartTime:    startTime,
						Duration:     elapsed,
					}:
					default:
					}
				case <-progressDone:
					return
				}
			}
		}()
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.GOMAXPROCS(0) * 3
	}
	sem := make(chan struct{}, concurrency)

	var visitedDirs sync.Map
	visitedDirs.Store(rootPath, true)

	var wg sync.WaitGroup
	s.scanDir(ctx, client, rootPath, root, opts, sem, &wg, &filesScanned, &dirsScanned, &bytesFound, &errCount, excludeSet, &visitedDirs)
	wg.Wait()

	updateSizeRecursive(root)

	if progress != nil {
		close(progressDone)
		progressWg.Wait()
		elapsed := time.Since(startTime)
		select {
		case progress <- scanner.Progress{
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

func (s *SFTPScanner) scanDir(
	ctx context.Context,
	client sftpClient,
	dirPath string,
	parent *model.DirNode,
	opts scanner.ScanOptions,
	sem chan struct{},
	wg *sync.WaitGroup,
	filesScanned, dirsScanned, bytesFound, errCount *atomic.Int64,
	excludeSet map[string]bool,
	visitedDirs *sync.Map,
) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	entries, err := client.ReadDir(dirPath)
	if err != nil {
		errCount.Add(1)
		return
	}

	dirsScanned.Add(1)

	spawnScan := func(path string, dir *model.DirNode) {
		select {
		case sem <- struct{}{}:
			wg.Add(1)
			go func(p string, d *model.DirNode) {
				defer wg.Done()
				defer func() { <-sem }()
				s.scanDir(ctx, client, p, d, opts, sem, wg, filesScanned, dirsScanned, bytesFound, errCount, excludeSet, visitedDirs)
			}(path, dir)
		default:
			s.scanDir(ctx, client, path, dir, opts, sem, wg, filesScanned, dirsScanned, bytesFound, errCount, excludeSet, visitedDirs)
		}
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		name := entry.Name()
		if excludeSet[name] {
			continue
		}
		if !opts.ShowHidden && isHidden(name) {
			continue
		}

		fullPath := cleanRemotePath(pathpkg.Join(dirPath, name))
		mode := entry.Mode()

		if mode&os.ModeSymlink != 0 {
			if opts.FollowSymlinks {
				resolvedPath, targetInfo, err := resolveSymlinkTarget(client, fullPath)
				if err != nil {
					errCount.Add(1)
					fileNode := &model.FileNode{
						Name:   name,
						Size:   0,
						Usage:  0,
						Mtime:  entry.ModTime(),
						Flag:   model.FlagSymlink | model.FlagError,
						Parent: parent,
					}
					parent.AddChild(fileNode)
					filesScanned.Add(1)
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

					if _, loaded := visitedDirs.LoadOrStore(resolvedPath, true); loaded {
						continue
					}
					spawnScan(resolvedPath, childDir)
					continue
				}

				size := targetInfo.Size()
				fileNode := &model.FileNode{
					Name:   name,
					Size:   size,
					Usage:  size,
					Mtime:  targetInfo.ModTime(),
					Flag:   model.FlagSymlink,
					Parent: parent,
				}
				parent.AddChild(fileNode)
				filesScanned.Add(1)
				bytesFound.Add(size)
				continue
			}

			size := entry.Size()
			fileNode := &model.FileNode{
				Name:   name,
				Size:   size,
				Usage:  size,
				Mtime:  entry.ModTime(),
				Flag:   model.FlagSymlink,
				Parent: parent,
			}
			parent.AddChild(fileNode)
			filesScanned.Add(1)
			bytesFound.Add(size)
			continue
		}

		if entry.IsDir() {
			scanPath := fullPath
			if resolvedPath, err := client.RealPath(fullPath); err == nil {
				scanPath = cleanRemotePath(resolvedPath)
			}

			childDir := &model.DirNode{
				FileNode: model.FileNode{
					Name:   name,
					Mtime:  entry.ModTime(),
					Parent: parent,
				},
			}
			parent.AddChild(childDir)

			if _, loaded := visitedDirs.LoadOrStore(scanPath, true); loaded {
				continue
			}
			spawnScan(scanPath, childDir)
			continue
		}

		size := entry.Size()
		fileNode := &model.FileNode{
			Name:   name,
			Size:   size,
			Usage:  size,
			Mtime:  entry.ModTime(),
			Parent: parent,
		}
		parent.AddChild(fileNode)
		filesScanned.Add(1)
		bytesFound.Add(size)
	}
}

func resolveSymlinkTarget(client sftpClient, symlinkPath string) (string, os.FileInfo, error) {
	target, err := client.ReadLink(symlinkPath)
	if err != nil {
		return "", nil, err
	}

	if !pathpkg.IsAbs(target) {
		target = pathpkg.Join(pathpkg.Dir(symlinkPath), target)
	}
	target = cleanRemotePath(target)

	resolvedPath, err := client.RealPath(target)
	if err != nil {
		return "", nil, err
	}
	resolvedPath = cleanRemotePath(resolvedPath)

	info, err := client.Stat(resolvedPath)
	if err != nil {
		return "", nil, err
	}

	return resolvedPath, info, nil
}

func cleanRemotePath(p string) string {
	if p == "" {
		return defaultRemotePath
	}
	clean := pathpkg.Clean(strings.ReplaceAll(p, "\\", "/"))
	if clean == "" {
		return defaultRemotePath
	}
	return clean
}

func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}

func updateSizeRecursive(dir *model.DirNode) {
	for _, child := range dir.Children {
		if cd, ok := child.(*model.DirNode); ok {
			updateSizeRecursive(cd)
		}
	}
	dir.UpdateSize()
}

func dialSFTP(_ context.Context, cfg Config) (sftpClient, io.Closer, error) {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, nil, fmt.Errorf("ssh port must be between 1 and 65535")
	}

	user, host, err := parseSSHTarget(cfg.Target)
	if err != nil {
		return nil, nil, err
	}

	hostCB, err := hostKeyCallback(host, cfg.Port, cfg.BatchMode)
	if err != nil {
		return nil, nil, err
	}

	auth, err := buildAuthMethods(user, host, cfg.BatchMode)
	if err != nil {
		return nil, nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostCB,
		Timeout:         15 * time.Second,
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", cfg.Port))
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, nil, fmt.Errorf("cannot start SFTP subsystem: %w", err)
	}

	closer := &remoteCloser{ssh: sshClient, sftp: sftpClient}
	return sftpClient, closer, nil
}

type remoteCloser struct {
	ssh  *ssh.Client
	sftp *sftp.Client
}

func (c *remoteCloser) Close() error {
	var retErr error
	if c.sftp != nil {
		if err := c.sftp.Close(); err != nil {
			retErr = err
		}
	}
	if c.ssh != nil {
		if err := c.ssh.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}
	return retErr
}
