package iosbackup

import (
	"context"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
)

// Device is a backup opened for scanning: every file it holds, laid out
// as one device (see disk.FileSet), with the backup's own folders.
type Device struct {
	*disk.FileSet
	vol *fsys.Volume
}

// fileMeta is what an MBFile tells besides dates.
type fileMeta struct {
	size int64  // -1 if not given
	key  []byte // the file's key, wrapped, in an encrypted backup
}

// Volumes returns the backup's files, read when it was opened.
func (d *Device) Volumes(context.Context, func(float64)) ([]*fsys.Volume, error) {
	return []*fsys.Volume{d.vol}, nil
}

// CarveRange is empty: the backup holds whole files.
func (d *Device) CarveRange() (int64, int64) { return 0, 0 }
