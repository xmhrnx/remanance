//go:build darwin && !ios

package disk

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
)

type fileDevice struct {
	f    *os.File
	r    *alignedReader
	info Info
}

var wholeDisk = regexp.MustCompile(`^disk[0-9]+$`)

var slice = regexp.MustCompile(`^disk[0-9]+s[0-9]+$`)

// readable are the file systems, as diskutil names them, whose
// partitions are listed to scan on their own; APFS and HFS+ aren't read,
// so their disks are scanned whole, by signature.
var readable = map[string]bool{"msdos": true, "exfat": true, "ntfs": true}

func openPhysical(info Info) (Device, error) {
	f, err := os.Open(info.Path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, ErrPermission
		}
		return nil, err
	}
	return &fileDevice{f: f, r: Aligned(f, info.SectorSize, info.Size).(*alignedReader), info: info}, nil
}

func (d *fileDevice) ReadAt(p []byte, off int64) (int, error) { return d.r.ReadAt(p, off) }

func (d *fileDevice) Info() Info { return d.info }

func (d *fileDevice) Size() int64 { return d.info.Size }

func (d *fileDevice) SectorSize() int { return d.info.SectorSize }

func (d *fileDevice) Close() error { return d.f.Close() }
