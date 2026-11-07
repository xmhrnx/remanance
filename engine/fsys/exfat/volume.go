package exfat

import (
	"context"
	"io"

	"github.com/xmhrnx/remanance/engine/fsys"
)

type volume struct {
	r      io.ReaderAt
	devOff int64
	b      boot
	fat    []byte
	bitmap fsys.Bitmap
	v      *fsys.Volume
	seen   map[uint32]bool
}

func init() { fsys.RegisterParser(Open, "exFAT") }

const maxDirBytes = 256 << 20 // exFAT's own limit for a directory

// Open parses the exFAT volume in r.
func Open(ctx context.Context, r io.ReaderAt, devOff, size int64, progress fsys.Progress) (*fsys.Volume, error) {
	s := make([]byte, 512)
	if _, err := r.ReadAt(s, 0); err != nil && err != io.EOF {
		return nil, err
	}
	b, err := parseBoot(s)
	if err != nil {
		// The backup boot region starts at sector 12.
		for _, ss := range []int64{512, 4096} {
			if _, e := r.ReadAt(s, 12*ss); e == nil {
				if b, err = parseBoot(s); err == nil {
					break
				}
			}
		}
		if err != nil {
			return nil, err
		}
	}
	vol := &volume{r: r, devOff: devOff, b: b, seen: map[uint32]bool{}}
	vol.fat = make([]byte, b.fatLength)
	r.ReadAt(vol.fat, b.fatOffset)
	vol.v = fsys.NewVolume("exFAT", devOff, size)
	vol.v.ClusterSize = b.clusterSize

	rootRaw := vol.readChain(b.rootCluster, maxDirBytes)
	_, bmCluster, bmLen, label := parseDir(rootRaw, b.clusterOffset(b.rootCluster))
	vol.v.Label = label
	if b.valid(bmCluster) && bmLen > 0 && bmLen < 1<<30 {
		vol.bitmap = fsys.Bitmap(vol.readChain(bmCluster, bmLen))
	} else {
		vol.v.Warn("the allocation bitmap is missing; recovery chances can't be estimated")
	}
	vol.seen[b.rootCluster] = true
	vol.walk(ctx, vol.v.Root, rootRaw, b.clusterOffset(b.rootCluster), false, 0)
	if progress != nil {
		progress(1)
	}
	vol.v.Sort()
	return vol.v, nil
}
