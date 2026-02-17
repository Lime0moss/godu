package ops

import (
	"encoding/json"
	"io"
	"os"
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
