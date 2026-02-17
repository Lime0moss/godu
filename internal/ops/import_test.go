package ops

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sadopc/godu/internal/model"
)

func TestImportJSON_RejectsUnexpectedChildElement(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	data := `[1,0,{"progname":"godu","progver":"dev","timestamp":0},[{"name":"/tmp/root"},123,{"name":"ok.txt","asize":1,"dsize":1}]]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ImportJSON(path)
	if err == nil {
		t.Fatal("expected malformed child element to fail import")
	}
	if !strings.Contains(err.Error(), "unexpected child element at index 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportJSON_RejectsTrailingGarbage(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "trailing.json")
	data := `[1,0,{"progname":"godu","progver":"dev","timestamp":0},[{"name":"/tmp/root"}]]
garbage`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ImportJSON(path)
	if err == nil {
		t.Fatal("expected trailing data to fail import")
	}
	if !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateName_SlashAlwaysRejected(t *testing.T) {
	if err := validateName("a/b"); err == nil {
		t.Fatal("expected slash to be rejected")
	}
}

func TestValidateName_BackslashAllowedOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash is a path separator on Windows")
	}
	if err := validateName(`a\b`); err != nil {
		t.Fatalf("expected backslash to be allowed on Unix, got: %v", err)
	}
}

func TestImportJSON_DirFlagsRoundTrip(t *testing.T) {
	root := &model.DirNode{
		FileNode: model.FileNode{Name: "/test-root"},
	}
	child := &model.DirNode{
		FileNode: model.FileNode{
			Name:   "symdir",
			Flag:   model.FlagSymlink | model.FlagError,
			Parent: root,
		},
	}
	child.AddChild(&model.FileNode{
		Name:   "file.txt",
		Size:   10,
		Usage:  10,
		Parent: child,
	})
	root.AddChild(child)
	root.UpdateSizeRecursive()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "flags.json")
	if err := ExportJSON(root, path, "test"); err != nil {
		t.Fatalf("export: %v", err)
	}

	imported, err := ImportJSON(path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	children := imported.GetChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(children))
	}
	dir, ok := children[0].(*model.DirNode)
	if !ok {
		t.Fatal("expected child to be a DirNode")
	}
	if dir.Flag&model.FlagSymlink == 0 {
		t.Error("expected FlagSymlink on imported dir")
	}
	if dir.Flag&model.FlagError == 0 {
		t.Error("expected FlagError on imported dir")
	}
}

func TestImportJSON_DepthLimit(t *testing.T) {
	// Build JSON with nesting > maxImportDepth
	var b strings.Builder
	b.WriteString(`[1,0,{"progname":"godu","progver":"dev","timestamp":0},`)
	for i := 0; i <= maxImportDepth+1; i++ {
		b.WriteString(`[{"name":"d"},`)
	}
	b.WriteString(`{"name":"f","asize":1}`)
	for i := 0; i <= maxImportDepth+1; i++ {
		b.WriteString(`]`)
	}
	b.WriteString(`]`)

	tmp := t.TempDir()
	path := filepath.Join(tmp, "deep.json")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ImportJSON(path)
	if err == nil {
		t.Fatal("expected depth limit error")
	}
	if !strings.Contains(err.Error(), "maximum depth") {
		t.Fatalf("unexpected error: %v", err)
	}
}
