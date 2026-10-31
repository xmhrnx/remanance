//go:build linux || darwin

package iphone

import "syscall"

// freeSpace is how many bytes the drive holding dir has free, for the
// phone to check before it backs up.
func freeSpace(dir string) int64 {
	var st syscall.Statfs_t
	if syscall.Statfs(dir, &st) != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
