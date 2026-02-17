package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Delete removes a file or directory at the given path.
// For directories, it removes the entire subtree.
// rootPath constrains deletion to descendants of the scan root.
//
// Symlinks themselves are safe to delete (os.Remove removes the link, not the target).
// However, paths that traverse through a symlinked directory are blocked to prevent
// deleting files outside the scan root.
func Delete(path string, rootPath string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve path %s: %w", path, err)
	}
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return fmt.Errorf("cannot resolve root %s: %w", rootPath, err)
	}

	// Resolve symlinks on the PARENT dir to catch traversal attacks,
	// while keeping the final component lexical (safe to delete symlinks themselves).
	realParent, err := filepath.EvalSymlinks(filepath.Dir(absPath))
	if err != nil {
		return fmt.Errorf("cannot resolve parent of %s: %w", absPath, err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fmt.Errorf("cannot resolve root %s: %w", absRoot, err)
	}

	realPath := filepath.Join(realParent, filepath.Base(absPath))

	// Ensure the target is strictly inside the root (not the root itself).
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to delete %s: outside scan root %s", absPath, absRoot)
	}

	info, err := os.Lstat(realPath)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", realPath, err)
	}

	if info.IsDir() {
		return os.RemoveAll(realPath)
	}
	return os.Remove(realPath)
}
