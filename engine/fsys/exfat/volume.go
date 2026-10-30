package exfat

import (
	"context"
	"encoding/binary"
	"io"

	"github.com/xmhrnx/remanance/engine/fsys"
)

func init() { fsys.RegisterParser(Open, "exFAT") }

const maxDirBytes = 256 << 20 // exFAT's own limit for a directory

type volume struct {
	r      io.ReaderAt
	devOff int64
	b      boot
	fat    []byte
	bitmap fsys.Bitmap
	v      *fsys.Volume
	seen   map[uint32]bool
}

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

func (vol *volume) next(c uint32) uint32 {
	i := int64(c) * 4
	if i+4 > int64(len(vol.fat)) {
		return 0
	}
	return binary.LittleEndian.Uint32(vol.fat[i:])
}

func (vol *volume) used(c uint32) bool {
	if vol.bitmap == nil {
		return true
	}
	return vol.bitmap.Used(int64(c) - 2)
}

// chain returns up to n clusters of a FAT chain from first.
func (vol *volume) chain(first uint32, n int64) []uint32 {
	var out []uint32
	seen := map[uint32]bool{}
	for c := first; vol.b.valid(c) && int64(len(out)) < n && !seen[c]; c = vol.next(c) {
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func (vol *volume) readClusters(cl []uint32, size int64) []byte {
	cs := vol.b.clusterSize
	out := make([]byte, int64(len(cl))*cs)
	for i, c := range cl {
		vol.r.ReadAt(out[int64(i)*cs:int64(i+1)*cs], vol.b.clusterOffset(c))
	}
	if size >= 0 && size < int64(len(out)) {
		out = out[:size]
	}
	return out
}

func (vol *volume) readChain(first uint32, size int64) []byte {
	cs := vol.b.clusterSize
	return vol.readClusters(vol.chain(first, (size+cs-1)/cs), size)
}

// clustersOf places a file's data: contiguous when the stream says so,
// else by its FAT chain. Deleting a file clears its bitmap bits but
// leaves the chain, unless the chain no longer adds up.
func (vol *volume) clustersOf(f fileSet) (cl []uint32, guessed bool) {
	cs := vol.b.clusterSize
	need := (f.size + cs - 1) / cs
	if !f.noChain {
		cl = vol.chain(f.cluster, need)
		if int64(len(cl)) == need {
			return cl, false
		}
	}
	cl = cl[:0]
	for c := f.cluster; int64(len(cl)) < need && vol.b.valid(c); c++ {
		cl = append(cl, c)
	}
	return cl, !f.noChain
}

func (vol *volume) walk(ctx context.Context, dir *fsys.Entry, raw []byte, base int64, parentDeleted bool, depth int) {
	if depth > 64 || ctx.Err() != nil {
		return
	}
	sets, _, _, _ := parseDir(raw, base)
	for _, f := range sets {
		e := &fsys.Entry{
			ID:       uint64(f.offset),
			Name:     f.name,
			IsDir:    f.isDir,
			Created:  f.created,
			Modified: f.modified,
			Accessed: f.accessed,
			Health:   100,
		}
		deleted := f.deleted || parentDeleted
		if deleted {
			e.State = fsys.Deleted
		}
		if f.isDir {
			if !vol.b.valid(f.cluster) || vol.seen[f.cluster] {
				continue
			}
			vol.seen[f.cluster] = true
			dir.Add(e)
			cl, _ := vol.clustersOf(f)
			vol.walk(ctx, e, vol.readClusters(cl, f.size), vol.b.clusterOffset(f.cluster), deleted, depth+1)
			continue
		}
		dir.Add(e)
		if f.size == 0 {
			continue
		}
		e.Size = f.size
		if !vol.b.valid(f.cluster) {
			e.Health, e.Note = 0, "The first cluster is unknown"
			continue
		}
		cl, guessed := vol.clustersOf(f)
		e.Runs = vol.runs(cl)
		if deleted {
			base := vol.devOff + vol.b.heapOffset - 2*vol.b.clusterSize
			e.Health = fsys.Health(e.Runs, vol.b.clusterSize, base, func(c int64) bool { return vol.used(uint32(c)) })
			if guessed && e.Health == 100 {
				e.Health = 90
				e.Note = "Placed contiguously; the cluster chain was gone"
			} else if e.Health < 100 {
				e.Note = "Some clusters have been reused since deletion"
			}
		}
	}
}

func (vol *volume) runs(cl []uint32) []fsys.Run {
	cs := vol.b.clusterSize
	var out []fsys.Run
	for i := 0; i < len(cl); {
		j := i + 1
		for j < len(cl) && cl[j] == cl[j-1]+1 {
			j++
		}
		out = append(out, fsys.Run{Offset: vol.devOff + vol.b.clusterOffset(cl[i]), Length: int64(j-i) * cs})
		i = j
	}
	return out
}
