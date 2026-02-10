package ops

import (
	"fmt"
	"os"
)

// Delete removes a file or directory at the given path.
// For directories, it removes the entire subtree.
func Delete(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", path, err)
	}

	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}
