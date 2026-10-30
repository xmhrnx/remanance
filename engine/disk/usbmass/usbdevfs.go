//go:build linux

package usbmass

import (
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/mehran9675/remanence/engine/disk"
)

// usbdevfs request structures, laid out as the kernel's: Go's alignment
// gives the same padding on 32- and 64-bit.
type bulkTransfer struct {
	ep, len, timeout uint32
	data             unsafe.Pointer
}

type ctrlTransfer struct {
	requestType, request uint8
	value, index, length uint16
	timeout              uint32
	data                 unsafe.Pointer
}

func ioc(dir, nr, size uintptr) uintptr { return dir<<30 | size<<16 | 'U'<<8 | nr }

var (
	ioctlControl   = ioc(3, 0, unsafe.Sizeof(ctrlTransfer{}))
	ioctlBulk      = ioc(3, 2, unsafe.Sizeof(bulkTransfer{}))
	ioctlClearHalt = ioc(2, 21, 4)
)

// DevFS is a Transport over a usbdevfs file descriptor: a device node
// under /dev/bus/usb, or the descriptor Android's UsbDeviceConnection
// gives an app the user let use the drive. The interface must already be
// claimed.
type DevFS struct {
	fd        int
	iface     uint16
	epIn      uint32
	epOut     uint32
	maxPacket int
}

// NewDevFS wraps fd. epIn and epOut are the bulk endpoint addresses
// (epIn has bit 7 set).
func NewDevFS(fd, iface, epIn, epOut int) *DevFS {
	return &DevFS{fd: fd, iface: uint16(iface), epIn: uint32(epIn), epOut: uint32(epOut)}
}

func (d *DevFS) ioctl(req uintptr, arg unsafe.Pointer) (int, error) {
	r, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(d.fd), req, uintptr(arg))
	if errno != 0 {
		return int(r), errno
	}
	return int(r), nil
}

func (d *DevFS) bulk(ep uint32, data []byte, timeout time.Duration) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	t := bulkTransfer{ep: ep, len: uint32(len(data)), timeout: uint32(timeout.Milliseconds()), data: unsafe.Pointer(&data[0])}
	n, err := d.ioctl(ioctlBulk, unsafe.Pointer(&t))
	runtime.KeepAlive(data)
	switch {
	case errors.Is(err, unix.EPIPE):
		return 0, ErrStall
	case errors.Is(err, unix.ETIMEDOUT):
		return 0, fmt.Errorf("usb: the drive stopped answering")
	case errors.Is(err, unix.ENODEV), errors.Is(err, unix.ESHUTDOWN):
		return 0, fmt.Errorf("usb: %w", disk.ErrGone)
	case err != nil:
		return 0, fmt.Errorf("usb: %w", err)
	}
	return n, nil
}

func (d *DevFS) BulkOut(data []byte, timeout time.Duration) (int, error) {
	return d.bulk(d.epOut, data, timeout)
}

func (d *DevFS) BulkIn(data []byte, timeout time.Duration) (int, error) {
	return d.bulk(d.epIn, data, timeout)
}

func (d *DevFS) ClearHalt(in bool) error {
	ep := d.epOut
	if in {
		ep = d.epIn
	}
	_, err := d.ioctl(ioctlClearHalt, unsafe.Pointer(&ep))
	return err
}

// Reset sends Bulk-Only Mass Storage Reset, a class request to the
// interface, then clears both endpoints as the specification asks.
func (d *DevFS) Reset() error {
	c := ctrlTransfer{requestType: 0x21, request: 0xff, index: d.iface, timeout: 5000}
	if _, err := d.ioctl(ioctlControl, unsafe.Pointer(&c)); err != nil {
		return fmt.Errorf("usb: reset: %w", err)
	}
	return errors.Join(d.ClearHalt(true), d.ClearHalt(false))
}

// MaxTransfer is 16 KiB: older kernels refuse bigger bulk URBs through
// usbdevfs, and it costs little speed.
func (d *DevFS) MaxTransfer() int { return 16 << 10 }
