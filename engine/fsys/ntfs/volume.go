package ntfs

import (
	"context"
	"io"

	"github.com/xmhrnx/remanance/engine/fsys"
)

const (
	recordMFT    = 0
	recordVolume = 3
	recordRoot   = 5
	recordBitmap = 6
	recordExtend = 11
	firstUser    = 16
)

// node is what the MFT pass keeps of each record.
type node struct {
	rec   *record
	name  fileName
	named bool
	si    standardInfo
	data  []attribute // unnamed $DATA pieces, from the base and extension records
	entry *fsys.Entry
}

type volume struct {
	r       io.ReaderAt
	devOff  int64
	size    int64
	b       boot
	mftRuns []run
	bitmap  fsys.Bitmap
	v       *fsys.Volume
}

func init() { fsys.RegisterParser(Open, "NTFS") }

// Open parses the NTFS volume in r.
func Open(ctx context.Context, r io.ReaderAt, devOff, size int64, progress fsys.Progress) (*fsys.Volume, error) {
	vol := &volume{r: r, devOff: devOff, size: size}
	sector := make([]byte, 512)
	if _, err := r.ReadAt(sector, 0); err != nil && err != io.EOF {
		return nil, err
	}
	b, err := parseBoot(sector)
	if err != nil {
		// Fall back to the backup boot sector in the last sector.
		if _, err2 := r.ReadAt(sector, size-512); err2 != nil {
			return nil, err
		}
		if b, err = parseBoot(sector); err != nil {
			return nil, err
		}
	}
	vol.b = b
	vol.v = fsys.NewVolume("NTFS", devOff, size)
	vol.v.ClusterSize = b.clusterSize

	if err := vol.loadMFTRuns(); err != nil {
		return nil, err
	}
	nodes, err := vol.readMFT(ctx, progress)
	if err != nil {
		return nil, err
	}
	vol.loadBitmap(nodes)
	vol.build(nodes)
	vol.v.Sort()
	return vol.v, nil
}
