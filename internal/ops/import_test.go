package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
