package model

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileNode_Path_Root(t *testing.T) {
	f := &FileNode{Name: "file.txt"}
	if got := f.Path(); got != "file.txt" {
		t.Errorf("Path() = %q, want %q", got, "file.txt")
	}
}

func TestFileNode_Path_Nested(t *testing.T) {
	root := &DirNode{FileNode: FileNode{Name: "/root"}}
	f := &FileNode{Name: "file.txt", Parent: root}
	want := filepath.Join("/root", "file.txt")
	if got := f.Path(); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestFileNode_Path_DeepNesting(t *testing.T) {
	root := &DirNode{FileNode: FileNode{Name: "/root"}}
	sub1 := &DirNode{FileNode: FileNode{Name: "a", Parent: root}}
	sub2 := &DirNode{FileNode: FileNode{Name: "b", Parent: sub1}}
	f := &FileNode{Name: "c.txt", Parent: sub2}
	want := filepath.Join("/root", "a", "b", "c.txt")
	if got := f.Path(); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestDirNode_AddChild_GetChildren(t *testing.T) {
	dir := &DirNode{FileNode: FileNode{Name: "parent"}}
	child1 := &FileNode{Name: "a.txt", Size: 10, Parent: dir}
	child2 := &FileNode{Name: "b.txt", Size: 20, Parent: dir}

	dir.AddChild(child1)
	dir.AddChild(child2)

	children := dir.GetChildren()
	if len(children) != 2 {
		t.Fatalf("GetChildren() returned %d items, want 2", len(children))
	}
	if children[0].GetName() != "a.txt" || children[1].GetName() != "b.txt" {
		t.Error("GetChildren() returned unexpected names")
	}

	// Verify GetChildren returns a copy (modifying returned slice shouldn't affect original)
	children[0] = nil
	orig := dir.GetChildren()
	if orig[0] == nil {
		t.Error("GetChildren() did not return a copy")
	}
}

func TestDirNode_UpdateSize(t *testing.T) {
	dir := &DirNode{FileNode: FileNode{Name: "parent"}}
	dir.AddChild(&FileNode{Name: "a", Size: 100, Usage: 200, Parent: dir})
	dir.AddChild(&FileNode{Name: "b", Size: 300, Usage: 400, Parent: dir})

	sub := &DirNode{FileNode: FileNode{Name: "sub", Parent: dir}}
	sub.AddChild(&FileNode{Name: "c", Size: 50, Usage: 60, Parent: sub})
	sub.UpdateSize()
	dir.AddChild(sub)

	dir.UpdateSize()

	if dir.Size != 450 {
		t.Errorf("Size = %d, want 450", dir.Size)
	}
	if dir.Usage != 660 {
		t.Errorf("Usage = %d, want 660", dir.Usage)
	}
	// ItemCount: 2 files + 1 subdir + 1 file in subdir = 4
	if dir.ItemCount != 4 {
		t.Errorf("ItemCount = %d, want 4", dir.ItemCount)
	}
}

func TestDirNode_RemoveChild(t *testing.T) {
	root := &DirNode{FileNode: FileNode{Name: "/root"}}
	a := &FileNode{Name: "a.txt", Size: 100, Usage: 100, Parent: root}
	b := &FileNode{Name: "b.txt", Size: 200, Usage: 200, Parent: root}
	root.AddChild(a)
	root.AddChild(b)
	root.UpdateSize()

	if !root.RemoveChild("a.txt") {
		t.Fatal("RemoveChild returned false for existing child")
	}
	if root.RemoveChild("nonexistent") {
		t.Fatal("RemoveChild returned true for non-existing child")
	}

	children := root.GetChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 child after removal, got %d", len(children))
	}
	// Size should be recalculated
	if root.Size != 200 {
		t.Errorf("Size after removal = %d, want 200", root.Size)
	}
}

func TestSortChildren(t *testing.T) {
	now := time.Now()
	children := []TreeNode{
		&FileNode{Name: "b.txt", Size: 10, Usage: 10, Mtime: now.Add(-1 * time.Hour)},
		&DirNode{FileNode: FileNode{Name: "a_dir", Size: 30, Usage: 30, Mtime: now}},
		&FileNode{Name: "c.txt", Size: 20, Usage: 20, Mtime: now.Add(-2 * time.Hour)},
	}

	// Sort by size descending, dirs first
	SortChildren(children, DefaultSort(), false)
	if children[0].GetName() != "a_dir" {
		t.Errorf("expected dir first, got %q", children[0].GetName())
	}
	if children[1].GetName() != "c.txt" || children[2].GetName() != "b.txt" {
		t.Error("expected files sorted by size descending")
	}

	// Sort by name ascending
	SortChildren(children, SortConfig{Field: SortByName, Order: SortAsc, DirsFirst: false}, false)
	if children[0].GetName() != "a_dir" || children[1].GetName() != "b.txt" || children[2].GetName() != "c.txt" {
		t.Error("expected items sorted by name ascending")
	}

	// Sort by mtime descending
	SortChildren(children, SortConfig{Field: SortByMtime, Order: SortDesc, DirsFirst: false}, false)
	if children[0].GetName() != "a_dir" { // most recent
		t.Errorf("expected most recent first, got %q", children[0].GetName())
	}
}

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		name string
		want FileCategory
	}{
		{"photo.jpg", CatMedia},
		{"PHOTO.JPG", CatMedia},
		{"main.go", CatCode},
		{"archive.tar.gz", CatArchive},
		{"readme.md", CatDocument},
		{"debug.log", CatSystem},
		{"program.exe", CatExecutable},
		{"noext", CatOther},
		{".hidden", CatOther},
	}

	for _, tt := range tests {
		got := ClassifyFile(tt.name)
		if got != tt.want {
			t.Errorf("ClassifyFile(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}
