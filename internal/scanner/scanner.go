package scanner

import (
	"context"

	"github.com/sadopc/godu/internal/model"
)

// ScanOptions configures the scanner behavior.
type ScanOptions struct {
	// ShowHidden includes hidden files/directories (starting with .)
	ShowHidden bool
	// FollowSymlinks follows symbolic links (default: false)
	FollowSymlinks bool
	// ExcludePatterns is a list of directory names to skip
	ExcludePatterns []string
	// DisableGC disables garbage collection during scan for speed
	DisableGC bool
	// Concurrency overrides the default semaphore count (0 = auto)
	Concurrency int
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() ScanOptions {
	return ScanOptions{
		ShowHidden:      true,
		FollowSymlinks:  false,
		ExcludePatterns: []string{},
		DisableGC:       false,
		Concurrency:     0,
	}
}

// Scanner is the interface for directory scanning.
type Scanner interface {
	// Scan scans the given path and returns the root DirNode.
	// Progress updates are sent on the progress channel.
	Scan(ctx context.Context, path string, opts ScanOptions, progress chan<- Progress) (*model.DirNode, error)
}

// ScanResult wraps the result of a scan operation.
type ScanResult struct {
	Root *model.DirNode
	Err  error
}
