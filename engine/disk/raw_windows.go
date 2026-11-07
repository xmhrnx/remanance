//go:build windows

package disk

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type windowsDevice struct {
	f    *os.File
	r    *alignedReader
	info Info
}

const (
	ioctlDiskGetDriveGeometryEx     = 0x000700A0
	ioctlStorageQueryProperty       = 0x002D1400
	ioctlVolumeGetVolumeDiskExtents = 0x00560000
	ioctlStorageGetDeviceNumber     = 0x002D1080
	fsctlAllowExtendedDasdIO        = 0x00090083
	maxPhysicalDrives               = 64
)

var windowsBusTypes = map[uint32]string{
	1: "SCSI", 2: "ATAPI", 3: "ATA", 4: "FireWire", 5: "SSA", 6: "Fibre", 7: "USB",
	8: "RAID", 9: "iSCSI", 10: "SAS", 11: "SATA", 12: "SD", 13: "MMC", 14: "Virtual",
	15: "Virtual", 16: "Spaces", 17: "NVMe", 18: "SCM", 19: "UFS",
}

func openPhysical(info Info) (Device, error) {
	h, err := openHandle(info.Path, windows.GENERIC_READ)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, ErrPermission
		}
		return nil, err
	}
	// Lets reads reach past the end a volume's file system reports.
	ioctl(h, fsctlAllowExtendedDasdIO, nil, nil)
	if info.Size == 0 {
		info.Size, info.SectorSize, _ = driveGeometry(h)
	}
	if info.SectorSize == 0 {
		info.SectorSize = 512
	}
	f := os.NewFile(uintptr(h), info.Path)
	return &windowsDevice{f: f, r: Aligned(f, info.SectorSize, info.Size).(*alignedReader), info: info}, nil
}

func (d *windowsDevice) ReadAt(p []byte, off int64) (int, error) { return d.r.ReadAt(p, off) }

func (d *windowsDevice) Info() Info { return d.info }

func (d *windowsDevice) Size() int64 { return d.info.Size }

func (d *windowsDevice) SectorSize() int { return d.info.SectorSize }

func (d *windowsDevice) Close() error { return d.f.Close() }
