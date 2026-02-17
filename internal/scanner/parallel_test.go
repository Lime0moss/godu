package scanner

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sadopc/godu/internal/model"
)

func TestScan_CanceledContext_ReturnsError(t *testing.T) {
	root := t.TempDir()
	// Create some files to scan
	for i := 0; i < 10; i++ {
		sub := filepath.Join(root, "dir"+string(rune('a'+i)))
		if err := os.Mkdir(sub, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "file.txt"), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	s := NewParallelScanner()
	result, err := s.Scan(ctx, root, ScanOptions{ShowHidden: true}, nil)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil root even on cancellation")
	}
}

func TestScan_CanceledContext_WithProgressChannel_DoesNotPanic(t *testing.T) {
	root := t.TempDir()

	progressCh := make(chan Progress, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := NewParallelScanner()
	_, err := s.Scan(ctx, root, ScanOptions{ShowHidden: true}, progressCh)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// If scanner goroutines are not fully stopped before return, the next ticker
	// send can panic after this close.
	close(progressCh)
	time.Sleep(120 * time.Millisecond)
}

func TestScan_NormalCompletion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewParallelScanner()
	result, err := s.Scan(context.Background(), root, ScanOptions{ShowHidden: true}, nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil root")
	}
	if len(result.GetChildren()) != 1 {
		t.Fatalf("expected 1 child, got %d", len(result.GetChildren()))
	}
}

func TestScan_ShowHiddenFalse_SkipsHiddenEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden.txt"), []byte("h"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".hidden-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden-dir", "inside.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewParallelScanner()
	result, err := s.Scan(context.Background(), root, ScanOptions{ShowHidden: false}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	names := map[string]bool{}
	for _, child := range result.GetChildren() {
		names[child.GetName()] = true
	}

	if !names["visible.txt"] {
		t.Fatal("expected visible file to be present")
	}
	if names[".hidden.txt"] {
		t.Fatal("expected hidden file to be skipped")
	}
	if names[".hidden-dir"] {
		t.Fatal("expected hidden directory to be skipped")
	}
}

func TestScan_FollowSymlinks_DedupsFileSymlinkAlias(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "target.txt")
	data := []byte("hello")
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(root, "alias.txt")
	if err := os.Symlink("target.txt", linkPath); err != nil {
		t.Skipf("symlink not available on this platform: %v", err)
	}

	s := NewParallelScanner()
	result, err := s.Scan(context.Background(), root, ScanOptions{ShowHidden: true, FollowSymlinks: true}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var targetNode, linkNode model.TreeNode
	for _, child := range result.GetChildren() {
		switch child.GetName() {
		case "target.txt":
			targetNode = child
		case "alias.txt":
			linkNode = child
		}
	}
	if targetNode == nil || linkNode == nil {
		t.Fatalf("expected both target and symlink nodes, got target=%v link=%v", targetNode != nil, linkNode != nil)
	}

	expected := int64(len(data))
	if result.GetSize() != expected {
		t.Fatalf("expected root apparent size %d, got %d", expected, result.GetSize())
	}

	nonZeroCount := 0
	hardlinkCount := 0
	for _, n := range []model.TreeNode{targetNode, linkNode} {
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

func TestScan_FollowSymlinks_BrokenSymlinkPlaceholder(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("/definitely/missing/target", filepath.Join(root, "broken-link")); err != nil {
		t.Skipf("symlink not available on this platform: %v", err)
	}

	s := NewParallelScanner()
	result, err := s.Scan(context.Background(), root, ScanOptions{ShowHidden: true, FollowSymlinks: true}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var broken model.TreeNode
	for _, child := range result.GetChildren() {
		if child.GetName() == "broken-link" {
			broken = child
			break
		}
	}
	if broken == nil {
		t.Fatal("expected broken symlink node to be present")
	}
	if broken.IsDir() {
		t.Fatal("expected broken symlink placeholder to be a file node")
	}
	if broken.GetSize() != 0 || broken.GetUsage() != 0 {
		t.Fatalf("expected zero-size placeholder, got size=%d usage=%d", broken.GetSize(), broken.GetUsage())
	}
	if broken.GetFlag()&model.FlagSymlink == 0 {
		t.Fatal("expected broken symlink placeholder to include FlagSymlink")
	}
	if broken.GetFlag()&model.FlagError == 0 {
		t.Fatal("expected broken symlink placeholder to include FlagError")
	}
}

func TestScan_PermissionDeniedDir_FlagError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0o000 not effective on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root — permission checks are bypassed")
	}

	root := t.TempDir()
	denied := filepath.Join(root, "noperm")
	if err := os.Mkdir(denied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(denied, "secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(denied, 0o755); err != nil {
			t.Logf("cleanup chmod failed for %s: %v", denied, err)
		}
	})

	s := NewParallelScanner()
	result, err := s.Scan(context.Background(), root, ScanOptions{ShowHidden: true}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var deniedNode model.TreeNode
	for _, child := range result.GetChildren() {
		if child.GetName() == "noperm" {
			deniedNode = child
			break
		}
	}
	if deniedNode == nil {
		t.Fatal("expected noperm directory to be present")
	}
	if deniedNode.GetFlag()&model.FlagError == 0 {
		t.Fatal("expected FlagError on permission-denied directory")
	}
}

func TestScan_SkipsUnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets not available on Windows")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "regular.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	sockPath := filepath.Join(root, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Skipf("cannot create unix socket: %v", err)
	}
	defer ln.Close()

	s := NewParallelScanner()
	result, err := s.Scan(context.Background(), root, ScanOptions{ShowHidden: true}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, child := range result.GetChildren() {
		if child.GetName() == "test.sock" {
			t.Fatal("expected unix socket to be filtered out")
		}
	}
	found := false
	for _, child := range result.GetChildren() {
		if child.GetName() == "regular.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected regular file to be present")
	}
}

func TestScan_FollowSymlinks_SkipsSymlinkToUnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets not available on Windows")
	}

	root := t.TempDir()

	sockPath := filepath.Join(root, "target.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Skipf("cannot create unix socket: %v", err)
	}
	defer ln.Close()

	linkPath := filepath.Join(root, "alias.sock")
	if err := os.Symlink("target.sock", linkPath); err != nil {
		t.Skipf("symlink not available on this platform: %v", err)
	}

	s := NewParallelScanner()
	result, err := s.Scan(context.Background(), root, ScanOptions{ShowHidden: true, FollowSymlinks: true}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, child := range result.GetChildren() {
		if child.GetName() == "alias.sock" {
			t.Fatal("expected symlink to unix socket to be filtered out")
		}
	}
}
