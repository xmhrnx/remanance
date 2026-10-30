package disk

import (
	"errors"
	"syscall"
)

// Windows errors for a drive that's no longer there. ERROR_DEV_NOT_EXIST
// is what a USB stick that dropped off the bus gives; ERROR_NOT_READY a
// card reader whose card was pulled.
var goneErrnos = []syscall.Errno{
	21,   // ERROR_NOT_READY
	55,   // ERROR_DEV_NOT_EXIST
	433,  // ERROR_NO_SUCH_DEVICE
	1167, // ERROR_DEVICE_NOT_CONNECTED
	1617, // ERROR_DEVICE_REMOVED
}

func goneErrno(err error) bool {
	var e syscall.Errno
	if !errors.As(err, &e) {
		return false
	}
	for _, g := range goneErrnos {
		if e == g {
			return true
		}
	}
	return false
}
