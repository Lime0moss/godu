package model

import (
	"context"
	"path/filepath"
	"sync"
	"time"
)

const (
	maxInt64 = int64(^uint64(0) >> 1)
	minInt64 = -maxInt64 - 1
)

// NodeFlag represents special file attributes.
type NodeFlag uint8

const (
	FlagNone    NodeFlag = 0
	FlagSymlink NodeFlag = 1 << iota
	FlagError
	FlagHardlink
	// FlagUsageEstimated marks nodes whose disk usage is estimated (not exact).
	FlagUsageEstimated
)

// FileNode represents a single file in the tree.
type FileNode struct {
	Name   string    // Relative name (not full path)
	Size   int64     // Apparent size in bytes
	Usage  int64     // Disk usage (blocks * block size)
	Mtime  time.Time // Last modification time
	Inode  uint64    // Inode number for hardlink detection
	Flag   NodeFlag
	Parent *DirNode // Parent directory (nil for root)
}

// DirNode represents a directory with children.
type DirNode struct {
	FileNode
	Children  []TreeNode // Mixed files and subdirectories
	ItemCount int64      // Total recursive item count
	mu        sync.RWMutex
}

// TreeNode is the interface satisfied by both FileNode and DirNode.
type TreeNode interface {
	GetName() string
	GetSize() int64
	GetUsage() int64
	GetMtime() time.Time
	GetFlag() NodeFlag
	GetParent() *DirNode
	IsDir() bool
	Path() string
}

// --- FileNode implements TreeNode ---

func (f *FileNode) GetName() string     { return f.Name }
func (f *FileNode) GetSize() int64      { return f.Size }
func (f *FileNode) GetUsage() int64     { return f.Usage }
func (f *FileNode) GetMtime() time.Time { return f.Mtime }
func (f *FileNode) GetFlag() NodeFlag   { return f.Flag }
func (f *FileNode) GetParent() *DirNode { return f.Parent }
func (f *FileNode) IsDir() bool         { return false }

func (f *FileNode) Path() string {
	return buildPath(f.Parent, f.Name)
}

// --- DirNode implements TreeNode ---

func (d *DirNode) IsDir() bool { return true }

func (d *DirNode) Path() string {
	return buildPath(d.Parent, d.Name)
}

// AddChild appends a child node thread-safely.
func (d *DirNode) AddChild(child TreeNode) {
	d.mu.Lock()
	d.Children = append(d.Children, child)
	d.mu.Unlock()
}

// GetChildren returns a snapshot of children thread-safely.
func (d *DirNode) GetChildren() []TreeNode {
	d.mu.RLock()
	cp := make([]TreeNode, len(d.Children))
	copy(cp, d.Children)
	d.mu.RUnlock()
	return cp
}

// UpdateSize recalculates this directory's size from its children.
func (d *DirNode) UpdateSize() {
	d.mu.RLock()
	var size, usage int64
	var count int64
	for _, c := range d.Children {
		size = saturatingAddInt64(size, c.GetSize())
		usage = saturatingAddInt64(usage, c.GetUsage())
		if cd, ok := c.(*DirNode); ok {
			count = saturatingAddInt64(count, cd.ItemCount)
		}
		count = saturatingAddInt64(count, 1)
	}
	d.mu.RUnlock()

	d.Size = size
	d.Usage = usage
	d.ItemCount = count
}

func saturatingAddInt64(a, b int64) int64 {
	if b > 0 && a > maxInt64-b {
		return maxInt64
	}
	if b < 0 && a < minInt64-b {
		return minInt64
	}
	return a + b
}

// RemoveChild removes a child by name and updates sizes up the tree.
func (d *DirNode) RemoveChild(name string) bool {
	d.mu.Lock()
	found := false
	for i, c := range d.Children {
		if c.GetName() == name {
			d.Children = append(d.Children[:i], d.Children[i+1:]...)
			found = true
			break
		}
	}
	d.mu.Unlock()

	if found {
		d.propagateSizeUpdate()
	}
	return found
}

// propagateSizeUpdate recalculates sizes from this node up to root.
func (d *DirNode) propagateSizeUpdate() {
	node := d
	for node != nil {
		node.UpdateSize()
		node = node.Parent
	}
}

// ReadChildren returns the children slice directly without copying.
// Safe for post-scan read-only access when no concurrent writes occur.
func (d *DirNode) ReadChildren() []TreeNode {
	return d.Children
}

// NewBrokenSymlinkNode creates a placeholder node for a broken symlink.
func NewBrokenSymlinkNode(name string, parent *DirNode) *FileNode {
	return &FileNode{
		Name:   name,
		Size:   0,
		Usage:  0,
		Flag:   FlagSymlink | FlagError,
		Parent: parent,
	}
}

// UpdateSizeRecursive performs a bottom-up size calculation.
// Children are updated before parents, ensuring correct totals.
// Must be called only after all concurrent writes are complete.
func (d *DirNode) UpdateSizeRecursive() {
	d.UpdateSizeRecursiveContext(context.Background())
}

// UpdateSizeRecursiveContext performs a bottom-up size calculation and stops
// early if the context is canceled.
func (d *DirNode) UpdateSizeRecursiveContext(ctx context.Context) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return
		default:
		}
	}

	for _, child := range d.Children {
		if cd, ok := child.(*DirNode); ok {
			cd.UpdateSizeRecursiveContext(ctx)
			if ctx != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}
	d.UpdateSize()
}

// buildPath reconstructs the full path by walking up the parent chain.
func buildPath(parent *DirNode, name string) string {
	if parent == nil {
		return name
	}
	depth := 0
	for p := parent; p != nil; p = p.Parent {
		depth++
	}
	parts := make([]string, depth+1)
	parts[depth] = name
	i := depth - 1
	for p := parent; p != nil; p = p.Parent {
		parts[i] = p.Name
		i--
	}
	return filepath.Join(parts...)
}
