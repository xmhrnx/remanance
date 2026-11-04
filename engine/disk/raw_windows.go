//go:build windows

package disk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

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

func openHandle(path string, access uint32) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
}

func ioctl(h windows.Handle, code uint32, in []byte, out []byte) (int, error) {
	var n uint32
	var inp, outp *byte
	if len(in) > 0 {
		inp = &in[0]
	}
	if len(out) > 0 {
		outp = &out[0]
	}
	err := windows.DeviceIoControl(h, code, inp, uint32(len(in)), outp, uint32(len(out)), &n, nil)
	return int(n), err
}

func driveGeometry(h windows.Handle) (size int64, sector int, err error) {
	out := make([]byte, 256)
	if _, err = ioctl(h, ioctlDiskGetDriveGeometryEx, nil, out); err != nil {
		return 0, 0, err
	}
	sector = int(binary.LittleEndian.Uint32(out[20:]))
	size = int64(binary.LittleEndian.Uint64(out[24:]))
	return size, sector, nil
}

func storageDescriptor(h windows.Handle, info *Info) {
	query := make([]byte, 12) // StorageDeviceProperty, PropertyStandardQuery
	out := make([]byte, 1024)
	n, err := ioctl(h, ioctlStorageQueryProperty, query, out)
	if err != nil || n < 32 {
		return
	}
	str := func(at int) string {
		o := int(binary.LittleEndian.Uint32(out[at:]))
		if o == 0 || o >= n {
			return ""
		}
		end := o
		for end < n && out[end] != 0 {
			end++
		}
		return strings.TrimSpace(string(out[o:end]))
	}
	info.Removable = out[10] != 0
	vendor, product := str(12), str(16)
	info.Model = strings.TrimSpace(vendor + " " + product)
	info.Serial = str(24)
	info.Bus = windowsBusTypes[binary.LittleEndian.Uint32(out[28:])]
	if info.Bus == "USB" || info.Bus == "SD" || info.Bus == "MMC" {
		info.Removable = true
	}
}

// volumeDisks maps each drive letter to the physical drive numbers it
// spans.
func volumeDisks() map[int][]string {
	out := map[int][]string{}
	mask, _ := windows.GetLogicalDrives()
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		letter := string(rune('A'+i)) + ":"
		h, err := openHandle(`\\.\`+letter, 0)
		if err != nil {
			continue
		}
		buf := make([]byte, 8+24*16)
		if n, err := ioctl(h, ioctlVolumeGetVolumeDiskExtents, nil, buf); err == nil && n >= 8 {
			count := int(binary.LittleEndian.Uint32(buf))
			for e := 0; e < count && 8+24*e+4 <= n; e++ {
				d := int(binary.LittleEndian.Uint32(buf[8+24*e:]))
				out[d] = append(out[d], letter)
			}
		}
		windows.CloseHandle(h)
	}
	return out
}

func listPhysical() ([]Info, error) {
	mounts := volumeDisks()
	system := strings.ToUpper(os.Getenv("SystemDrive"))
	var out []Info
	for i := 0; i < maxPhysicalDrives; i++ {
		path := fmt.Sprintf(`\\.\PhysicalDrive%d`, i)
		h, err := openHandle(path, 0)
		if err != nil {
			continue
		}
		size, sector, err := driveGeometry(h)
		if err != nil || size == 0 {
			windows.CloseHandle(h)
			continue
		}
		info := Info{
			ID:         path,
			Path:       path,
			Kind:       KindPhysical,
			Size:       size,
			SectorSize: sector,
			Mounts:     mounts[i],
		}
		storageDescriptor(h, &info)
		windows.CloseHandle(h)
		for _, m := range info.Mounts {
			if strings.EqualFold(m, system) {
				info.System = true
			}
		}
		info.Name = info.Model
		if info.Name == "" {
			info.Name = fmt.Sprintf("Disk %d", i)
		}
		out = append(out, info)
	}
	return append(out, listVolumes(system)...), nil
}

// listVolumes lists the drive letters on local drives, so one can be
// scanned on its own. Their size and drive come from the extents, which
// asking for needs no rights.
func listVolumes(system string) []Info {
	var out []Info
	mask, _ := windows.GetLogicalDrives()
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		letter := string(rune('A'+i)) + ":"
		root, _ := windows.UTF16PtrFromString(letter + `\`)
		if t := windows.GetDriveType(root); t != windows.DRIVE_FIXED && t != windows.DRIVE_REMOVABLE {
			continue
		}
		path := `\\.\` + letter
		h, err := openHandle(path, 0)
		if err != nil {
			continue
		}
		info := Info{ID: path, Path: path, Kind: KindVolume, Mounts: []string{letter}}
		buf := make([]byte, 8+24*16)
		if n, err := ioctl(h, ioctlVolumeGetVolumeDiskExtents, nil, buf); err == nil && n >= 8 {
			count := int(binary.LittleEndian.Uint32(buf))
			for e := 0; e < count && 8+24*e+24 <= n; e++ {
				info.Size += int64(binary.LittleEndian.Uint64(buf[8+24*e+16:]))
			}
		}
		_, info.SectorSize, _ = driveGeometry(h)
		storageDescriptor(h, &info)
		windows.CloseHandle(h)
		if info.Size == 0 {
			continue
		}
		label := make([]uint16, windows.MAX_PATH+1)
		if windows.GetVolumeInformation(root, &label[0], uint32(len(label)), nil, nil, nil, nil, 0) == nil {
			info.Name = windows.UTF16ToString(label)
		}
		if info.Name == "" {
			info.Name = "Local Disk"
		}
		info.Name += " (" + letter + ")"
		info.System = strings.EqualFold(letter, system)
		out = append(out, info)
	}
	return out
}

type windowsDevice struct {
	f    *os.File
	r    *alignedReader
	info Info
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
func (d *windowsDevice) Info() Info                              { return d.info }
func (d *windowsDevice) Size() int64                             { return d.info.Size }
func (d *windowsDevice) SectorSize() int                         { return d.info.SectorSize }
func (d *windowsDevice) Close() error                            { return d.f.Close() }
