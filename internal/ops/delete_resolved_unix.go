//go:build !windows

package ops

import (
	"errors"
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

func deleteResolvedPath(parentPath, baseName string) error {
	parentFD, err := unix.Open(parentPath, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(parentFD)

	return deleteAt(parentFD, baseName)
}

// deleteAt removes name relative to parentFD without following symlinks.
func deleteAt(parentFD int, name string) error {
	// Fast path for files/symlinks.
	if err := unix.Unlinkat(parentFD, name, 0); err == nil {
		return nil
	} else if !errors.Is(err, unix.EISDIR) && !errors.Is(err, unix.EPERM) {
		if errors.Is(err, unix.ENOENT) {
			return fs.ErrNotExist
		}
		return err
	}

	// Directory path: open without following symlinks, recursively delete children.
	childFD, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return fs.ErrNotExist
		}
		// Entry may have changed type concurrently. Retry file/symlink unlink once.
		if errors.Is(err, unix.ENOTDIR) {
			if unlinkErr := unix.Unlinkat(parentFD, name, 0); unlinkErr == nil {
				return nil
			} else if errors.Is(unlinkErr, unix.ENOENT) {
				return fs.ErrNotExist
			} else {
				return unlinkErr
			}
		}
		return err
	}

	childDir := os.NewFile(uintptr(childFD), name)
	entries, readErr := childDir.ReadDir(-1)
	if readErr != nil {
		_ = childDir.Close()
		return readErr
	}

	for _, entry := range entries {
		if err := deleteAt(childFD, entry.Name()); err != nil {
			_ = childDir.Close()
			return err
		}
	}

	if err := childDir.Close(); err != nil {
		return err
	}

	if err := unix.Unlinkat(parentFD, name, unix.AT_REMOVEDIR); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return fs.ErrNotExist
		}
		return err
	}
	return nil
}
