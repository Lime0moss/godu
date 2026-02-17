//go:build windows

package ops

import (
	"os"
	"path/filepath"
)

func deleteResolvedPath(parentPath, baseName string) error {
	realPath := filepath.Join(parentPath, baseName)
	info, err := os.Lstat(realPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(realPath)
	}
	return os.Remove(realPath)
}
