//go:build unix

package cleanup

import (
	"os"
	"syscall"
)

func free(path string) (Space, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Space{}, err
	}
	return Space{Free: int64(st.Bavail) * int64(st.Bsize), Total: int64(st.Blocks) * int64(st.Bsize)}, nil
}

// mine says whether the user owns the file at path.
func mine(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return !ok || int(st.Uid) == os.Getuid()
}
