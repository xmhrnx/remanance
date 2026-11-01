//go:build unix

package space

import (
	"io/fs"
	"syscall"
)

// deviceOf is the drive a file is on.
func deviceOf(info fs.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Dev)
	}
	return 0
}

// allocated is the space a file takes on the drive: less than its size
// when it's sparse, more when its last block is part empty.
func allocated(info fs.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Blocks > 0 {
		return int64(st.Blocks) * 512
	}
	return info.Size()
}
