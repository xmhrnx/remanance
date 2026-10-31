package iphone

import (
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// freeSpace is how many bytes the drive holding dir has free, for the
// phone to check before it backs up.
func freeSpace(dir string) int64 {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0
	}
	var free uint64
	if r, _, _ := getDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&free)), 0, 0); r == 0 {
		return 0
	}
	return int64(free)
}
