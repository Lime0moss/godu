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

const defaultRemoteBlockSize int64 = 4096
const maxInt64 = int64(^uint64(0) >> 1)

// Config configures a remote SFTP scan.
type Config struct {
	Target      string
	Port        int
	BatchMode   bool
	Timeout     time.Duration
	ScanTimeout time.Duration
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

var dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

var sshNewClientConn = func(conn net.Conn, addr string, config *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	return ssh.NewClientConn(conn, addr, config)
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

	if s.cfg.ScanTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.ScanTimeout)
		defer cancel()
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
			Flag:  model.FlagUsageEstimated,
		},
	}

	excludeSet := make(map[string]struct{}, len(opts.ExcludePatterns))
	for _, p := range opts.ExcludePatterns {
		excludeSet[p] = struct{}{}
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
	if progress != nil {
		// Always stop the progress goroutine before returning, including cancel/error paths.
		defer func() {
			close(progressDone)
			progressWg.Wait()
		}()
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.GOMAXPROCS(0) * 3
	}
	sem := make(chan struct{}, concurrency)
	blockSize := remoteBlockSize(client, rootPath)

	var visitedDirs sync.Map
	visitedDirs.Store(rootPath, true)
	var seenFiles sync.Map

	var wg sync.WaitGroup
	s.scanDir(ctx, client, rootPath, rootPath, root, opts, sem, &wg, &filesScanned, &dirsScanned, &bytesFound, &errCount, excludeSet, &visitedDirs, &seenFiles, blockSize)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return root, err
	}
	root.UpdateSizeRecursiveContext(ctx)

	if progress != nil {
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
	scanRoot string,
	dirPath string,
	parent *model.DirNode,
	opts scanner.ScanOptions,
	sem chan struct{},
	wg *sync.WaitGroup,
	filesScanned, dirsScanned, bytesFound, errCount *atomic.Int64,
	excludeSet map[string]struct{},
	visitedDirs *sync.Map,
	seenFiles *sync.Map,
	blockSize int64,
) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	entries, err := readRemoteDir(ctx, client, dirPath)
	if err != nil {
		parent.Flag |= model.FlagError
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
				s.scanDir(ctx, client, scanRoot, p, d, opts, sem, wg, filesScanned, dirsScanned, bytesFound, errCount, excludeSet, visitedDirs, seenFiles, blockSize)
			}(path, dir)
		default:
			s.scanDir(ctx, client, scanRoot, path, dir, opts, sem, wg, filesScanned, dirsScanned, bytesFound, errCount, excludeSet, visitedDirs, seenFiles, blockSize)
		}
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		name := entry.Name()
		if _, excluded := excludeSet[name]; excluded {
			continue
		}
		if !opts.ShowHidden && isHidden(name) {
			continue
		}

		fullPath := cleanRemotePath(pathpkg.Join(dirPath, name))
		mode := entry.Mode()
		if isSpecialRemoteMode(mode) {
			continue
		}

		if mode&os.ModeSymlink != 0 {
			if opts.FollowSymlinks {
				resolvedPath, targetInfo, err := resolveSymlinkTarget(client, fullPath)
				if err != nil {
					errCount.Add(1)
					node := model.NewBrokenSymlinkNode(name, parent)
					node.Mtime = entry.ModTime()
					parent.AddChild(node)
					filesScanned.Add(1)
					continue
				}
				if isSpecialRemoteMode(targetInfo.Mode()) {
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

					// Skip symlinks pointing inside the scan root (will be scanned via normal traversal)
					if isWithinRemote(scanRoot, resolvedPath) {
						continue
					}

					if _, loaded := visitedDirs.LoadOrStore(resolvedPath, true); loaded {
						continue
					}
					spawnScan(resolvedPath, childDir)
					continue
				}

				size := targetInfo.Size()
				flag := model.FlagSymlink
				if _, loaded := seenFiles.LoadOrStore(resolvedPath, true); loaded {
					flag |= model.FlagHardlink
					size = 0
				}
				fileNode := &model.FileNode{
					Name:   name,
					Size:   size,
					Usage:  estimateDiskUsage(size, blockSize),
					Mtime:  targetInfo.ModTime(),
					Flag:   flag,
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
				Usage:  estimateDiskUsage(size, blockSize),
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
		flag := model.NodeFlag(0)
		if opts.FollowSymlinks {
			canonicalPath := fullPath
			if resolvedPath, err := client.RealPath(fullPath); err == nil {
				canonicalPath = cleanRemotePath(resolvedPath)
			}
			if _, loaded := seenFiles.LoadOrStore(canonicalPath, true); loaded {
				flag |= model.FlagHardlink
				size = 0
			}
		}
		fileNode := &model.FileNode{
			Name:   name,
			Size:   size,
			Usage:  estimateDiskUsage(size, blockSize),
			Mtime:  entry.ModTime(),
			Flag:   flag,
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

func estimateDiskUsage(size, blockSize int64) int64 {
	if size <= 0 {
		return 0
	}
	if blockSize <= 0 {
		blockSize = defaultRemoteBlockSize
	}
	blocks := (size + blockSize - 1) / blockSize
	return blocks * blockSize
}

func remoteBlockSize(client sftpClient, rootPath string) int64 {
	vfsClient, ok := client.(interface {
		StatVFS(path string) (*sftp.StatVFS, error)
	})
	if !ok {
		return defaultRemoteBlockSize
	}

	stat, err := vfsClient.StatVFS(rootPath)
	if err != nil || stat == nil {
		return defaultRemoteBlockSize
	}

	if stat.Frsize > 0 && stat.Frsize <= uint64(maxInt64) {
		return int64(stat.Frsize)
	}
	if stat.Bsize > 0 && stat.Bsize <= uint64(maxInt64) {
		return int64(stat.Bsize)
	}
	return defaultRemoteBlockSize
}

func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}

// isWithinRemote checks whether target is inside root using POSIX path semantics.
func isWithinRemote(root, target string) bool {
	root = pathpkg.Clean(root)
	target = pathpkg.Clean(target)
	if root == target {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return strings.HasPrefix(target, prefix)
}

func isSpecialRemoteMode(mode os.FileMode) bool {
	return mode&(os.ModeDevice|os.ModeCharDevice|os.ModeSocket|os.ModeNamedPipe|os.ModeIrregular) != 0
}

func readRemoteDir(ctx context.Context, client sftpClient, dirPath string) ([]os.FileInfo, error) {
	if rc, ok := client.(interface {
		ReadDirContext(context.Context, string) ([]os.FileInfo, error)
	}); ok {
		return rc.ReadDirContext(ctx, dirPath)
	}
	return client.ReadDir(dirPath)
}

func dialSFTP(ctx context.Context, cfg Config) (sftpClient, io.Closer, error) {
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

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostCB,
		Timeout:         timeout,
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", cfg.Port))
	sshClient, err := connectSSH(dialCtx, addr, sshConfig)
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

func connectSSH(ctx context.Context, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	conn, err := dialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	// Ensure cancellation interrupts handshake/authentication.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	c, chans, reqs, err := sshNewClientConn(conn, addr, config)
	close(done)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
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
