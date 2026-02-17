package ops

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sadopc/godu/internal/model"
)

func TestExportJSON_Stdout(t *testing.T) {
	root := &model.DirNode{FileNode: model.FileNode{Name: "/root"}}
	root.AddChild(&model.FileNode{
		Name:   "file.txt",
		Size:   12,
		Usage:  4096,
		Parent: root,
	})
	root.UpdateSize()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	os.Stdout = w

	exportErr := ExportJSON(root, "-", "test-version")
	closeErr := w.Close()
	os.Stdout = oldStdout

	if exportErr != nil {
		t.Fatalf("ExportJSON returned error: %v", exportErr)
	}
	if closeErr != nil {
		t.Fatalf("closing pipe writer failed: %v", closeErr)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	out := strings.TrimSpace(string(data))
	if !strings.Contains(out, `"progver":"test-version"`) {
		t.Fatalf("expected version in export output, got:\n%s", out)
	}
	if !strings.Contains(out, `"name":"file.txt"`) {
		t.Fatalf("expected file entry in export output, got:\n%s", out)
	}

	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("export output is not valid JSON: %v\n%s", err, out)
	}
	if len(raw) < 4 {
		t.Fatalf("expected ncdu format array with >=4 elements, got %d", len(raw))
	}
}

func TestExportJSON_AtomicNoPartialFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "output.json")

	// Export a valid tree — file should exist after success
	root := &model.DirNode{FileNode: model.FileNode{Name: "/root"}}
	root.AddChild(&model.FileNode{Name: "a.txt", Size: 1, Usage: 1, Parent: root})
	root.UpdateSize()

	if err := ExportJSON(root, target, "test"); err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	// Re-import to verify valid JSON
	reimported, err := ImportJSON(target)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if reimported.GetSize() != 1 {
		t.Fatalf("expected size 1, got %d", reimported.GetSize())
	}
}

func TestExportJSON_DirFlags(t *testing.T) {
	root := &model.DirNode{FileNode: model.FileNode{Name: "/root"}}
	root.AddChild(&model.DirNode{
		FileNode: model.FileNode{
			Name:   "errdir",
			Flag:   model.FlagError,
			Parent: root,
		},
	})
	root.UpdateSizeRecursive()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "flags.json")
	if err := ExportJSON(root, path, "test"); err != nil {
		t.Fatalf("export: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"read_error":true`) {
		t.Fatalf("expected read_error flag in export: %s", data)
	}
}

func TestExportJSON_OverwriteExistingFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "scan.json")

	rootA := &model.DirNode{FileNode: model.FileNode{Name: "/root"}}
	rootA.AddChild(&model.FileNode{Name: "a.txt", Size: 1, Usage: 1, Parent: rootA})
	rootA.UpdateSizeRecursive()
	if err := ExportJSON(rootA, path, "test"); err != nil {
		t.Fatalf("first export failed: %v", err)
	}

	rootB := &model.DirNode{FileNode: model.FileNode{Name: "/root"}}
	rootB.AddChild(&model.FileNode{Name: "b.txt", Size: 7, Usage: 7, Parent: rootB})
	rootB.UpdateSizeRecursive()
	if err := ExportJSON(rootB, path, "test"); err != nil {
		t.Fatalf("second export failed: %v", err)
	}

	imported, err := ImportJSON(path)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if imported.GetSize() != 7 {
		t.Fatalf("expected overwritten export size 7, got %d", imported.GetSize())
	}

	children := imported.GetChildren()
	if len(children) != 1 || children[0].GetName() != "b.txt" {
		t.Fatalf("expected overwritten export to contain b.txt, got %+v", children)
	}
}
