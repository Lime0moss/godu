package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	pathpkg "path"
	"testing"
	"time"

	"github.com/sadopc/godu/internal/model"
	"github.com/sadopc/godu/internal/scanner"
	"golang.org/x/crypto/ssh"
)

func TestScanWithClient_FiltersHiddenAndExcluded(t *testing.T) {
	client := newFakeSFTP(map[string]fakeNode{
		"/root":                  {mode: os.ModeDir, children: []string{"keep", "skip", ".hidden", "file.txt", "link"}},
		"/root/keep":             {mode: os.ModeDir, children: []string{"inside.txt"}},
		"/root/keep/inside.txt":  {mode: 0, size: 5},
		"/root/skip":             {mode: os.ModeDir, children: []string{"ignored.txt"}},
		"/root/skip/ignored.txt": {mode: 0, size: 9},
		"/root/.hidden":          {mode: 0, size: 11},
		"/root/file.txt":         {mode: 0, size: 7},
		"/root/link":             {mode: os.ModeSymlink, size: 3, target: "/root/file.txt"},
	})

	s := &SFTPScanner{cfg: Config{Target: "user@host", Port: 22}, dial: fakeDial(client)}
	root, err := s.Scan(context.Background(), "/root", scanner.ScanOptions{
		ShowHidden:      false,
		FollowSymlinks:  false,
		ExcludePatterns: []string{"skip"},
	}, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if got := findNode(root, ".hidden"); got != nil {
		t.Fatal("expected hidden file to be filtered")
	}
	if got := findNode(root, "skip"); got != nil {
		t.Fatal("expected excluded directory to be filtered")
	}

	file := findNode(root, "file.txt")
	if file == nil {
		t.Fatal("expected file.txt")
	}
	if file.GetSize() != 7 {
		t.Fatalf("unexpected file size: %d", file.GetSize())
	}

	link := findNode(root, "link")
	if link == nil {
		t.Fatal("expected symlink placeholder")
	}
	if link.GetFlag()&model.FlagSymlink == 0 {
		t.Fatal("expected symlink flag")
	}
}

func TestScanWithClient_FollowSymlinkDirDedups(t *testing.T) {
	client := newFakeSFTP(map[string]fakeNode{
		"/root":              {mode: os.ModeDir, children: []string{"dir", "dir-link"}},
		"/root/dir":          {mode: os.ModeDir, children: []string{"item.txt"}},
		"/root/dir/item.txt": {mode: 0, size: 10},
		"/root/dir-link":     {mode: os.ModeSymlink, target: "/root/dir"},
	})

	s := &SFTPScanner{cfg: Config{Target: "user@host", Port: 22}, dial: fakeDial(client)}
	root, err := s.Scan(context.Background(), "/root", scanner.ScanOptions{
		ShowHidden:     true,
		FollowSymlinks: true,
	}, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	link := findNode(root, "dir-link")
	if link == nil || !link.IsDir() {
		t.Fatal("expected dir-link directory node")
	}
	if link.GetFlag()&model.FlagSymlink == 0 {
		t.Fatal("expected dir-link symlink flag")
	}

	if root.GetSize() != 10 {
		t.Fatalf("expected root size dedup to be 10, got %d", root.GetSize())
	}
}

func TestScanWithClient_FollowSymlinkFileDedupsAlias(t *testing.T) {
	client := newFakeSFTP(map[string]fakeNode{
		"/root":            {mode: os.ModeDir, children: []string{"target.txt", "alias.txt"}},
		"/root/target.txt": {mode: 0, size: 10},
		"/root/alias.txt":  {mode: os.ModeSymlink, target: "/root/target.txt"},
	})

	s := &SFTPScanner{cfg: Config{Target: "user@host", Port: 22}, dial: fakeDial(client)}
	root, err := s.Scan(context.Background(), "/root", scanner.ScanOptions{
		ShowHidden:     true,
		FollowSymlinks: true,
	}, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	var targetNode, aliasNode model.TreeNode
	for _, child := range root.GetChildren() {
		switch child.GetName() {
		case "target.txt":
			targetNode = child
		case "alias.txt":
			aliasNode = child
		}
	}
	if targetNode == nil || aliasNode == nil {
		t.Fatalf("expected target and alias nodes, got target=%v alias=%v", targetNode != nil, aliasNode != nil)
	}

	if root.GetSize() != 10 {
		t.Fatalf("expected root size 10, got %d", root.GetSize())
	}

	nonZeroCount := 0
	hardlinkCount := 0
	for _, n := range []model.TreeNode{targetNode, aliasNode} {
		if n.GetSize() > 0 {
			nonZeroCount++
		}
		if n.GetFlag()&model.FlagHardlink != 0 {
			hardlinkCount++
		}
	}
	if nonZeroCount != 1 {
		t.Fatalf("expected exactly one non-zero node size, got %d", nonZeroCount)
	}
	if hardlinkCount != 1 {
		t.Fatalf("expected exactly one hardlink-marked node, got %d", hardlinkCount)
	}
}

func TestScanWithClient_BrokenSymlinkGetsErrorFlag(t *testing.T) {
	client := newFakeSFTP(map[string]fakeNode{
		"/root":        {mode: os.ModeDir, children: []string{"broken"}},
		"/root/broken": {mode: os.ModeSymlink, target: "/missing"},
	})

	s := &SFTPScanner{cfg: Config{Target: "user@host", Port: 22}, dial: fakeDial(client)}
	root, err := s.Scan(context.Background(), "/root", scanner.ScanOptions{
		ShowHidden:     true,
		FollowSymlinks: true,
	}, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	broken := findNode(root, "broken")
	if broken == nil {
		t.Fatal("expected broken symlink node")
	}
	if broken.GetFlag()&model.FlagSymlink == 0 {
		t.Fatal("expected symlink flag")
	}
	if broken.GetFlag()&model.FlagError == 0 {
		t.Fatal("expected error flag")
	}
}

func TestScanWithClient_ReadDirError_SetsFlagError(t *testing.T) {
	client := newFakeSFTP(map[string]fakeNode{
		"/root":        {mode: os.ModeDir, children: []string{"denied"}},
		"/root/denied": {mode: os.ModeDir, errOnRead: true},
	})

	s := &SFTPScanner{cfg: Config{Target: "user@host", Port: 22}, dial: fakeDial(client)}
	root, err := s.Scan(context.Background(), "/root", scanner.ScanOptions{
		ShowHidden: true,
	}, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	denied := findNode(root, "denied")
	if denied == nil {
		t.Fatal("expected denied dir node")
	}
	if denied.GetFlag()&model.FlagError == 0 {
		t.Fatal("expected FlagError on permission-denied directory")
	}
}

func TestScanWithClient_UsageUsesBlockEstimate(t *testing.T) {
	client := newFakeSFTP(map[string]fakeNode{
		"/root":          {mode: os.ModeDir, children: []string{"tiny.txt"}},
		"/root/tiny.txt": {mode: 0, size: 1},
	})

	s := &SFTPScanner{cfg: Config{Target: "user@host", Port: 22}, dial: fakeDial(client)}
	root, err := s.Scan(context.Background(), "/root", scanner.ScanOptions{
		ShowHidden: true,
	}, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	tiny := findNode(root, "tiny.txt")
	if tiny == nil {
		t.Fatal("expected tiny.txt node")
	}
	if tiny.GetSize() != 1 {
		t.Fatalf("expected size 1, got %d", tiny.GetSize())
	}
	if tiny.GetUsage() != remoteBlockSize {
		t.Fatalf("expected usage %d, got %d", remoteBlockSize, tiny.GetUsage())
	}
	if root.GetUsage() != remoteBlockSize {
		t.Fatalf("expected root usage %d, got %d", remoteBlockSize, root.GetUsage())
	}
}

func TestScanWithClient_SymlinkInsideScanRoot_NotDoubleScanned(t *testing.T) {
	// dir-link points to /root/dir (inside scan root) — should skip recursion
	client := newFakeSFTP(map[string]fakeNode{
		"/root":              {mode: os.ModeDir, children: []string{"dir", "dir-link"}},
		"/root/dir":          {mode: os.ModeDir, children: []string{"item.txt"}},
		"/root/dir/item.txt": {mode: 0, size: 10},
		"/root/dir-link":     {mode: os.ModeSymlink, target: "/root/dir"},
	})

	s := &SFTPScanner{cfg: Config{Target: "user@host", Port: 22}, dial: fakeDial(client)}
	root, err := s.Scan(context.Background(), "/root", scanner.ScanOptions{
		ShowHidden:     true,
		FollowSymlinks: true,
	}, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// Size should be 10, not 20 (no double-counting)
	if root.GetSize() != 10 {
		t.Fatalf("expected root size 10 (no double-count), got %d", root.GetSize())
	}
}

func TestScanWithClient_SkipsSpecialFiles(t *testing.T) {
	client := newFakeSFTP(map[string]fakeNode{
		"/root":             {mode: os.ModeDir, children: []string{"regular.txt", "pipe"}},
		"/root/regular.txt": {mode: 0, size: 4},
		"/root/pipe":        {mode: os.ModeNamedPipe},
	})

	s := &SFTPScanner{cfg: Config{Target: "user@host", Port: 22}, dial: fakeDial(client)}
	root, err := s.Scan(context.Background(), "/root", scanner.ScanOptions{
		ShowHidden: true,
	}, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if findNode(root, "pipe") != nil {
		t.Fatal("expected named pipe to be skipped")
	}
	if findNode(root, "regular.txt") == nil {
		t.Fatal("expected regular file to be present")
	}
}

func TestIsWithinRemote(t *testing.T) {
	tests := []struct {
		root, target string
		want         bool
	}{
		{"/root", "/root", true},
		{"/root", "/root/sub", true},
		{"/root", "/root/sub/deep", true},
		{"/root", "/other", false},
		{"/root", "/rootmore", false},
		{"/root", "/roo", false},
		{"/root", "/..", false},
	}
	for _, tt := range tests {
		got := isWithinRemote(tt.root, tt.target)
		if got != tt.want {
			t.Errorf("isWithinRemote(%q, %q) = %v, want %v", tt.root, tt.target, got, tt.want)
		}
	}
}

func TestEstimateDiskUsage(t *testing.T) {
	tests := []struct {
		size int64
		want int64
	}{
		{size: 0, want: 0},
		{size: -1, want: 0},
		{size: 1, want: remoteBlockSize},
		{size: remoteBlockSize, want: remoteBlockSize},
		{size: remoteBlockSize + 1, want: 2 * remoteBlockSize},
	}

	for _, tt := range tests {
		if got := estimateDiskUsage(tt.size); got != tt.want {
			t.Fatalf("estimateDiskUsage(%d) = %d, want %d", tt.size, got, tt.want)
		}
	}
}

func TestConnectSSH_RespectsContextCancellation(t *testing.T) {
	origDial := dialContext
	origNewClientConn := sshNewClientConn
	t.Cleanup(func() {
		dialContext = origDial
		sshNewClientConn = origNewClientConn
	})

	dialCalled := false
	handshakeCalled := false

	dialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		dialCalled = true
		<-ctx.Done()
		return nil, ctx.Err()
	}
	sshNewClientConn = func(net.Conn, string, *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
		handshakeCalled = true
		return nil, nil, nil, errors.New("unexpected handshake call")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := connectSSH(ctx, "example.com:22", &ssh.ClientConfig{
		User:            "user",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if !dialCalled {
		t.Fatal("expected dial to be called")
	}
	if handshakeCalled {
		t.Fatal("did not expect SSH handshake to start after canceled dial")
	}
}

func fakeDial(client sftpClient) func(context.Context, Config) (sftpClient, io.Closer, error) {
	return func(context.Context, Config) (sftpClient, io.Closer, error) {
		return client, noopCloser{}, nil
	}
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

func findNode(root *model.DirNode, parts ...string) model.TreeNode {
	if root == nil {
		return nil
	}

	var node model.TreeNode = root
	for _, part := range parts {
		dir, ok := node.(*model.DirNode)
		if !ok {
			return nil
		}

		var next model.TreeNode
		for _, child := range dir.GetChildren() {
			if child.GetName() == part {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		node = next
	}

	return node
}

type fakeNode struct {
	mode      os.FileMode
	size      int64
	mtime     time.Time
	target    string
	children  []string
	errOnRead bool // if true, ReadDir returns an error
}

type fakeSFTP struct {
	nodes map[string]fakeNode
}

func newFakeSFTP(nodes map[string]fakeNode) *fakeSFTP {
	cp := make(map[string]fakeNode, len(nodes))
	for k, v := range nodes {
		if v.mtime.IsZero() {
			v.mtime = time.Unix(1700000000, 0)
		}
		cp[cleanRemotePath(k)] = v
	}
	return &fakeSFTP{nodes: cp}
}

func (f *fakeSFTP) ReadDir(path string) ([]os.FileInfo, error) {
	node, err := f.get(path)
	if err != nil {
		return nil, err
	}
	if !node.mode.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	if node.errOnRead {
		return nil, fmt.Errorf("permission denied")
	}

	out := make([]os.FileInfo, 0, len(node.children))
	for _, child := range node.children {
		childPath := cleanRemotePath(pathpkg.Join(cleanRemotePath(path), child))
		childNode, ok := f.nodes[childPath]
		if !ok {
			return nil, fmt.Errorf("missing child %s", childPath)
		}
		out = append(out, fakeInfo{name: child, size: childNode.size, mode: childNode.mode, mtime: childNode.mtime})
	}
	return out, nil
}

func (f *fakeSFTP) Stat(path string) (os.FileInfo, error) {
	resolved, err := f.RealPath(path)
	if err != nil {
		return nil, err
	}
	node, ok := f.nodes[resolved]
	if !ok {
		return nil, os.ErrNotExist
	}
	return fakeInfo{name: pathpkg.Base(resolved), size: node.size, mode: node.mode, mtime: node.mtime}, nil
}

func (f *fakeSFTP) ReadLink(path string) (string, error) {
	node, err := f.get(path)
	if err != nil {
		return "", err
	}
	if node.mode&os.ModeSymlink == 0 {
		return "", fmt.Errorf("not symlink")
	}
	return node.target, nil
}

func (f *fakeSFTP) RealPath(path string) (string, error) {
	clean := cleanRemotePath(path)
	return f.resolve(clean, map[string]bool{})
}

func (f *fakeSFTP) get(path string) (fakeNode, error) {
	node, ok := f.nodes[cleanRemotePath(path)]
	if !ok {
		return fakeNode{}, os.ErrNotExist
	}
	return node, nil
}

func (f *fakeSFTP) resolve(path string, seen map[string]bool) (string, error) {
	node, ok := f.nodes[path]
	if !ok {
		return "", os.ErrNotExist
	}
	if node.mode&os.ModeSymlink == 0 {
		return path, nil
	}
	if seen[path] {
		return "", fmt.Errorf("symlink cycle")
	}
	seen[path] = true

	target := node.target
	if !pathpkg.IsAbs(target) {
		target = pathpkg.Join(pathpkg.Dir(path), target)
	}
	return f.resolve(cleanRemotePath(target), seen)
}

type fakeInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	mtime time.Time
}

func (fi fakeInfo) Name() string       { return fi.name }
func (fi fakeInfo) Size() int64        { return fi.size }
func (fi fakeInfo) Mode() os.FileMode  { return fi.mode }
func (fi fakeInfo) ModTime() time.Time { return fi.mtime }
func (fi fakeInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi fakeInfo) Sys() any           { return nil }
