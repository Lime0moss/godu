package ui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sadopc/godu/internal/model"
	"github.com/sadopc/godu/internal/scanner"
)

func TestAppFatalError_SetOnScanDoneError(t *testing.T) {
	app := NewApp("/tmp", scanner.DefaultOptions())
	scanErr := errors.New("scan failed")

	_, cmd := app.Update(ScanDoneMsg{Err: scanErr})
	if !errors.Is(app.FatalError(), scanErr) {
		t.Fatalf("expected fatal error %v, got %v", scanErr, app.FatalError())
	}
	if cmd == nil {
		t.Fatal("expected quit command on scan error")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestAppFatalError_NotSetByStatusMessages(t *testing.T) {
	app := NewApp("/tmp", scanner.DefaultOptions())

	_, _ = app.Update(ExportDoneMsg{Path: "out.json"})
	if app.FatalError() != nil {
		t.Fatalf("expected nil fatal error, got %v", app.FatalError())
	}
	if app.statusMsg == "" {
		t.Fatal("expected status message to be set for successful export")
	}
}

func TestAppMarkedSize_ComputesFromVisibleItems(t *testing.T) {
	app := NewApp("/tmp", scanner.DefaultOptions())
	root := &model.DirNode{FileNode: model.FileNode{Name: "/tmp/root"}}
	fileA := &model.FileNode{
		Name:   "a.txt",
		Size:   10,
		Usage:  20,
		Parent: root,
	}
	fileB := &model.FileNode{
		Name:   "b.txt",
		Size:   4,
		Usage:  8,
		Parent: root,
	}

	app.marked = map[string]bool{
		fileA.Path():            true,
		"/tmp/root/missing.txt": true, // Marked but not visible in current items
	}

	items := []model.TreeNode{fileA, fileB}

	app.useApparent = false
	if got := app.markedSize(items); got != 20 {
		t.Fatalf("expected disk marked size 20, got %d", got)
	}

	app.useApparent = true
	if got := app.markedSize(items); got != 10 {
		t.Fatalf("expected apparent marked size 10, got %d", got)
	}
}
