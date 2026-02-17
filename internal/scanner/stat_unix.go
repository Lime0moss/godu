//go:build !windows

package scanner

import (
	"os"
	"syscall"
)

// statInfo holds platform-specific file metadata.
type statInfo struct {
	inode     uint64
	dev       uint64
	diskUsage int64
	nlink     uint64
	ok        bool // true if platform stat was available
}

// getStatInfo extracts inode, device, disk usage, and nlink from file info.
func getStatInfo(info os.FileInfo) statInfo {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return statInfo{diskUsage: info.Size()}
	}
	return statInfo{
		inode:     stat.Ino,
		dev:       uint64(stat.Dev),
		diskUsage: int64(stat.Blocks) * 512,
		nlink:     uint64(stat.Nlink),
		ok:        true,
	}
}
