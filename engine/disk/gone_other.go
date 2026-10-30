//go:build !unix && !windows

package disk

func goneErrno(error) bool { return false }
