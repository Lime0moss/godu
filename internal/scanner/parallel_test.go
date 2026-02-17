package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScan_CanceledContext_ReturnsError(t *testing.T) {
	root := t.TempDir()
	// Create some files to scan
	for i := 0; i < 10; i++ {
		sub := filepath.Join(root, "dir"+string(rune('a'+i)))
		os.Mkdir(sub, 0755)
		os.WriteFile(filepath.Join(sub, "file.txt"), []byte("data"), 0644)
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

func TestScan_NormalCompletion(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0644)

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
