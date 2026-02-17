package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sadopc/godu/internal/model"
	"github.com/sadopc/godu/internal/ops"
)

const helperEnvKey = "GO_WANT_GODU_HELPER_PROCESS"

type cliResult struct {
	stdout   string
	stderr   string
	exitCode int
}

type nodeSnapshot struct {
	IsDir bool
	Size  int64
	Usage int64
	Flag  model.NodeFlag
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv(helperEnvKey) != "1" {
		return
	}

	sep := -1
	for i, arg := range os.Args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 {
		fmt.Fprintln(os.Stderr, "missing -- argument separator for helper process")
		os.Exit(2)
	}

	os.Args = append([]string{os.Args[0]}, os.Args[sep+1:]...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	main()
	os.Exit(0)
}

func TestE2E_HeadlessExportImportRoundTrip(t *testing.T) {
	scanRoot := createScanFixture(t)
	exportPath := filepath.Join(t.TempDir(), "scan.json")

	result := runCLI(t, "--export", exportPath, scanRoot)
	if result.exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.exitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "Exported to "+exportPath) {
		t.Fatalf("expected export confirmation in stdout, got:\n%s", result.stdout)
	}

	imported, err := ops.ImportJSON(exportPath)
	if err != nil {
		t.Fatalf("importing exported JSON failed: %v", err)
	}

	nested := findNode(imported, "keep", "sub", "b.go")
	if nested == nil {
		t.Fatal("expected keep/sub/b.go to exist in imported tree")
	}

	wantPath := filepath.Join(imported.Name, "keep", "sub", "b.go")
	if nested.Path() != wantPath {
		t.Fatalf("unexpected reconstructed path: got %q want %q", nested.Path(), wantPath)
	}

	if findNode(imported, ".hidden.txt") == nil {
		t.Fatal("expected hidden file to be present in default export")
	}

	// Verify symlink flag survives export/import round-trip
	linkNode := findNode(imported, "keep", "link.txt")
	if linkNode == nil {
		t.Fatal("expected keep/link.txt symlink to exist in imported tree")
	}
	if linkNode.GetFlag()&model.FlagSymlink == 0 {
		t.Fatal("expected FlagSymlink to be preserved after export/import round-trip")
	}

	reExportPath := filepath.Join(t.TempDir(), "rescan.json")
	result = runCLI(t, "--import", exportPath, "--export", reExportPath)
	if result.exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.exitCode, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "Exported to "+reExportPath) {
		t.Fatalf("expected re-export confirmation in stdout, got:\n%s", result.stdout)
	}

	reImported, err := ops.ImportJSON(reExportPath)
	if err != nil {
		t.Fatalf("importing re-exported JSON failed: %v", err)
	}

	if got, want := snapshotTree(reImported), snapshotTree(imported); !reflect.DeepEqual(got, want) {
		t.Fatalf("tree snapshot mismatch after import/export round trip\ngot:  %v\nwant: %v", got, want)
	}
}

func TestE2E_HeadlessExportHonorsExcludePatterns(t *testing.T) {
	scanRoot := createScanFixture(t)
	exportPath := filepath.Join(t.TempDir(), "scan.json")

	result := runCLI(t, "--exclude", "skip-one, skip-two", "--export", exportPath, scanRoot)
	if result.exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.exitCode, result.stdout, result.stderr)
	}

	imported, err := ops.ImportJSON(exportPath)
	if err != nil {
		t.Fatalf("importing excluded export failed: %v", err)
	}

	if findNode(imported, "skip-one") != nil {
		t.Fatal("expected skip-one directory to be excluded from scan")
	}
	if findNode(imported, "skip-two") != nil {
		t.Fatal("expected skip-two directory to be excluded from scan")
	}
	if findNode(imported, "keep") == nil {
		t.Fatal("expected keep directory to remain in scan output")
	}
}

func TestE2E_ImportExportFailsWhenImportFileMissing(t *testing.T) {
	missingImport := filepath.Join(t.TempDir(), "missing.json")
	exportPath := filepath.Join(t.TempDir(), "out.json")

	result := runCLI(t, "--import", missingImport, "--export", exportPath)
	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit for missing import file\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stderr, "Error importing:") {
		t.Fatalf("expected import error message, got:\n%s", result.stderr)
	}
	if _, err := os.Stat(exportPath); !os.IsNotExist(err) {
		t.Fatalf("expected no output file, stat err=%v", err)
	}
}

func TestE2E_HeadlessExportToStdoutWritesJSONOnly(t *testing.T) {
	scanRoot := createScanFixture(t)

	result := runCLI(t, "--export", "-", scanRoot)
	if result.exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s", result.exitCode, result.stdout, result.stderr)
	}
	if strings.Contains(result.stdout, "Scanning ") {
		t.Fatalf("expected stdout to contain only JSON, got:\n%s", result.stdout)
	}
	if strings.Contains(result.stdout, "Exported to") {
		t.Fatalf("expected stdout to contain only JSON, got:\n%s", result.stdout)
	}
	if strings.TrimSpace(result.stderr) != "" {
		t.Fatalf("expected empty stderr, got:\n%s", result.stderr)
	}

	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.stdout)), &raw); err != nil {
		t.Fatalf("expected valid JSON in stdout, got error: %v\nstdout:\n%s", err, result.stdout)
	}
	if len(raw) < 4 {
		t.Fatalf("expected ncdu root array, got %d elements", len(raw))
	}
}

func TestE2E_ImportRejectsScanTargets(t *testing.T) {
	importPath := filepath.Join(t.TempDir(), "scan.json")

	result := runCLI(t, "--import", importPath, "alice@10.0.0.2")
	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit code\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stderr, "--import cannot be used with scan targets") {
		t.Fatalf("unexpected error message:\n%s", result.stderr)
	}
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()

	cmdArgs := append([]string{"-test.run=^TestCLIHelperProcess$", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), helperEnvKey+"=1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := cliResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}

	if err == nil {
		return result
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("failed to execute helper process: %v", err)
	}

	result.exitCode = exitErr.ExitCode()
	return result
}

func createScanFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	mustMkdirAll(t, filepath.Join(root, "keep", "sub"))
	mustMkdirAll(t, filepath.Join(root, "skip-one"))
	mustMkdirAll(t, filepath.Join(root, "skip-two"))

	mustWriteFile(t, filepath.Join(root, "keep", "a.txt"), "alpha")
	mustWriteFile(t, filepath.Join(root, "keep", "sub", "b.go"), "package main\n")
	mustWriteFile(t, filepath.Join(root, "skip-one", "ignored.log"), "ignore me")
	mustWriteFile(t, filepath.Join(root, "skip-two", "ignored.log"), "ignore me too")
	mustWriteFile(t, filepath.Join(root, ".hidden.txt"), "top secret")

	// Symlink for round-trip metadata test
	if err := os.Symlink(filepath.Join(root, "keep", "a.txt"), filepath.Join(root, "keep", "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	return root
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

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

func snapshotTree(root *model.DirNode) map[string]nodeSnapshot {
	out := make(map[string]nodeSnapshot)

	var walk func(dir *model.DirNode, rel string)
	walk = func(dir *model.DirNode, rel string) {
		out[rel] = nodeSnapshot{
			IsDir: true,
			Size:  dir.GetSize(),
			Usage: dir.GetUsage(),
			Flag:  dir.GetFlag(),
		}

		for _, child := range dir.GetChildren() {
			childRel := child.GetName()
			if rel != "." {
				childRel = filepath.Join(rel, child.GetName())
			}

			if subdir, ok := child.(*model.DirNode); ok {
				walk(subdir, childRel)
				continue
			}

			out[childRel] = nodeSnapshot{
				IsDir: false,
				Size:  child.GetSize(),
				Usage: child.GetUsage(),
				Flag:  child.GetFlag(),
			}
		}
	}

	walk(root, ".")
	return out
}
