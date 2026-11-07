//go:build linux && !android

package disk

import (
	"errors"
	"io/fs"
	"os"
)

type fileDevice struct {
	f    *os.File
	info Info
}

func openPhysical(info Info) (Device, error) {
	f, err := os.Open(info.Path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, ErrPermission
		}
		return nil, err
	}
	return &fileDevice{f: f, info: info}, nil
}

func (d *fileDevice) ReadAt(p []byte, off int64) (int, error) { return d.f.ReadAt(p, off) }

func (d *fileDevice) Info() Info { return d.info }

func (d *fileDevice) Size() int64 { return d.info.Size }

func (d *fileDevice) SectorSize() int { return d.info.SectorSize }

func (d *fileDevice) Close() error { return d.f.Close() }
