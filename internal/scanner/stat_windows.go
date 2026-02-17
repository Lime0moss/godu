//go:build windows

package scanner

import "os"

// statInfo holds platform-specific file metadata.
type statInfo struct {
	inode     uint64
	dev       uint64
	diskUsage int64
	nlink     uint64
	ok        bool // true if platform stat was available
}

// getStatInfo on Windows falls back to apparent size for disk usage.
// Inode/hardlink detection is not supported.
func getStatInfo(info os.FileInfo) statInfo {
	return statInfo{diskUsage: info.Size()}
}
