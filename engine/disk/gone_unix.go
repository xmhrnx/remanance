//go:build unix

package disk

import (
	"errors"
	"syscall"
)

// goneErrno reports the errors Linux and macOS give for a drive that's
// no longer there.
func goneErrno(err error) bool {
	return errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENXIO) || errors.Is(err, syscall.ESHUTDOWN)
}
