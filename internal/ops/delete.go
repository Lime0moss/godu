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
func Delete(path string, rootPath string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve path %s: %w", path, err)
	}
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return fmt.Errorf("cannot resolve root %s: %w", rootPath, err)
	}

	// Ensure the target is strictly inside the root (not the root itself).
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("refusing to delete %s: outside scan root %s", absPath, absRoot)
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", absPath, err)
	}

	if info.IsDir() {
		return os.RemoveAll(absPath)
	}
	return os.Remove(absPath)
}
