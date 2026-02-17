package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDelete_NormalFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "file.txt")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Delete(f, root); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if _, err := os.Lstat(f); !os.IsNotExist(err) {
		t.Fatal("file should have been deleted")
	}
}

func TestDelete_Directory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "subdir")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Delete(dir, root); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("directory should have been deleted")
	}
}

func TestDelete_RootItself_Blocked(t *testing.T) {
	root := t.TempDir()
	err := Delete(root, root)
	if err == nil {
		t.Fatal("deleting root itself should be blocked")
	}
}

func TestDelete_OutsideRoot_Blocked(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Delete(target, root)
	if err == nil {
		t.Fatal("deleting outside root should be blocked")
	}
	// Verify file still exists
	if _, err := os.Lstat(target); err != nil {
		t.Fatal("file outside root should not have been deleted")
	}
}

func TestDelete_DotDotTraversal_Blocked(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "..", "shouldnotdelete.txt")
	// We just need to verify the path check blocks it
	err := Delete(outside, root)
	if err == nil {
		t.Fatal("dot-dot traversal should be blocked")
	}
}

func TestDelete_SymlinkInsideRoot_DeletesLink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "mylink")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	// Deleting a symlink inside root should succeed (removes the link, not the target)
	if err := Delete(link, root); err != nil {
		t.Fatalf("expected success deleting symlink, got %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal("symlink should have been deleted")
	}
	// Target should still exist
	if _, err := os.Lstat(target); err != nil {
		t.Fatal("target of symlink should still exist")
	}
}

func TestDelete_ThroughSymlinkDir_Blocked(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink dir inside root that points to outside
	symlinkDir := filepath.Join(root, "escape")
	if err := os.Symlink(outside, symlinkDir); err != nil {
		t.Fatal(err)
	}

	// Try to delete a file through the symlinked directory
	throughPath := filepath.Join(root, "escape", "secret.txt")
	err := Delete(throughPath, root)
	if err == nil {
		t.Fatal("deleting through symlink dir should be blocked")
	}
	// File should still exist
	if _, err := os.Lstat(target); err != nil {
		t.Fatal("file should not have been deleted through symlink")
	}
}

func TestDelete_BrokenSymlink_Deleted(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "broken")
	if err := os.Symlink("/nonexistent/path/that/doesnt/exist", link); err != nil {
		t.Fatal(err)
	}

	if err := Delete(link, root); err != nil {
		t.Fatalf("expected success deleting broken symlink, got %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal("broken symlink should have been deleted")
	}
}

func TestDelete_DotDotInName_Allowed(t *testing.T) {
	root := t.TempDir()
	// A file named "..foo" is NOT a traversal — it's a valid filename
	f := filepath.Join(root, "..foo")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Delete(f, root); err != nil {
		t.Fatalf("file named ..foo should be deletable, got %v", err)
	}
	if _, err := os.Lstat(f); !os.IsNotExist(err) {
		t.Fatal("..foo should have been deleted")
	}
}

func TestDelete_NestedFile(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(sub, "deep.txt")
	if err := os.WriteFile(f, []byte("deep"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Delete(f, root); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if _, err := os.Lstat(f); !os.IsNotExist(err) {
		t.Fatal("nested file should have been deleted")
	}
}
