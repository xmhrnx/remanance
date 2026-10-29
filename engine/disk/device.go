// Package disk gives the recovery engine read-only access to storage:
// physical drives, partitions, USB mass-storage devices and disk image
// files. Every source is exposed as a Device, a sized io.ReaderAt.
package disk

import (
	"errors"
	"io"
)

// Kind tells where a device's bytes come from.
type Kind int

const (
	KindPhysical Kind = iota // a whole drive
	KindVolume               // a mounted volume or partition
	KindImage                // a disk image file
	KindUSB                  // a USB drive reached without the OS (Android OTG)
)

func (k Kind) String() string {
	switch k {
	case KindPhysical:
		return "Drive"
	case KindVolume:
		return "Volume"
	case KindImage:
		return "Image"
	case KindUSB:
		return "USB drive"
	}
	return "Unknown"
}

// Info describes a device before it is opened.
type Info struct {
	ID         string // stable key, also what Open takes
	Path       string // OS path (\\.\PhysicalDrive1, /dev/sdb, a file)
	Name       string // what the user sees
	Model      string
	Serial     string
	Bus        string // "USB", "SATA", "NVMe", "SD", ...
	Kind       Kind
	Removable  bool
	System     bool // holds the running OS
	Size       int64
	SectorSize int
	Mounts     []string // drive letters or mount points on it
}

// Device is an open, read-only source of sectors.
type Device interface {
	io.ReaderAt
	Info() Info
	Size() int64
	SectorSize() int
	Close() error
}

var (
	// ErrPermission means the OS refused raw access; on desktops the app
	// must run elevated.
	ErrPermission = errors.New("disk: raw access needs administrator rights")
	// ErrNotSupported means this platform can't open that kind of device.
	ErrNotSupported = errors.New("disk: not supported on this platform")
)

// ReadFull reads exactly len(p) bytes at off, treating a short read at the
// end of the device as io.ErrUnexpectedEOF.
func ReadFull(d io.ReaderAt, p []byte, off int64) error {
	n, err := d.ReadAt(p, off)
	if n == len(p) {
		return nil
	}
	if err == nil || err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}
