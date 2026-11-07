package fat

import (
	"context"
	"fmt"
	"io"

	"github.com/xmhrnx/remanance/engine/fsys"
)

type volume struct {
	r      io.ReaderAt
	devOff int64
	b      bpb
	t      *table
	v      *fsys.Volume
	// dirs maps a folder's first cluster to its entry, so orphan search
	// skips folders the tree already has.
	dirs map[uint32]*fsys.Entry
}

// maxDirClusters bounds a directory's size: 65536 entries of 32 bytes.
const maxDirBytes = 65536 * 32

func init() { fsys.RegisterParser(Open, "FAT12", "FAT16", "FAT32") }

// Open parses the FAT volume in r.
func Open(ctx context.Context, r io.ReaderAt, devOff, size int64, progress fsys.Progress) (*fsys.Volume, error) {
	s := make([]byte, 512)
	if _, err := r.ReadAt(s, 0); err != nil && err != io.EOF {
		return nil, err
	}
	b, err := parseBPB(s)
	if err != nil {
		// FAT32 keeps a backup boot sector, usually at sector 6.
		if _, err2 := r.ReadAt(s, 6*512); err2 == nil {
			b, err = parseBPB(s)
		}
		if err != nil {
			return nil, err
		}
	}
	t, err := readTable(r, &b)
	if err != nil {
		return nil, fmt.Errorf("fat: reading the allocation table: %w", err)
	}
	vol := &volume{r: r, devOff: devOff, b: b, t: t, dirs: map[uint32]*fsys.Entry{}}
	vol.v = fsys.NewVolume(fmt.Sprintf("FAT%d", b.bits), devOff, size)
	vol.v.Label = b.label
	vol.v.ClusterSize = b.clusterSize

	root := vol.v.Root
	var raw []byte
	if b.bits == 32 {
		root.ID = uint64(b.rootCluster)
		raw = vol.readChain(b.rootCluster)
		vol.dirs[b.rootCluster] = root
	} else {
		raw = make([]byte, b.rootSize)
		r.ReadAt(raw, b.rootOffset)
	}
	base := b.rootOffset
	if b.bits == 32 {
		base = b.clusterOffset(b.rootCluster)
	}
	vol.walk(ctx, root, raw, base, 0)
	if progress != nil {
		progress(0.5)
	}
	if fsys.IsDeep(ctx) {
		vol.findOrphans(ctx, progress)
	}
	if progress != nil {
		progress(1)
	}
	vol.v.Sort()
	return vol.v, nil
}

// readChain reads an existing directory by following its cluster chain.
func (vol *volume) readChain(first uint32) []byte {
	cs := vol.b.clusterSize
	chain := vol.t.chain(first, maxDirBytes/cs)
	out := make([]byte, int64(len(chain))*cs)
	for i, c := range chain {
		vol.r.ReadAt(out[int64(i)*cs:int64(i+1)*cs], vol.b.clusterOffset(c))
	}
	return out
}

// readDeletedDir reads a deleted directory: its chain is gone, so take
// contiguous clusters from the first one while they still look like
// directory entries.
func (vol *volume) readDeletedDir(first uint32) []byte {
	cs := vol.b.clusterSize
	var out []byte
	buf := make([]byte, cs)
	for c := first; vol.b.validCluster(c) && int64(len(out)) < maxDirBytes; c++ {
		if c != first && vol.t.used(c) {
			break
		}
		if _, err := vol.r.ReadAt(buf, vol.b.clusterOffset(c)); err != nil {
			break
		}
		if c == first && !(buf[0] == '.' && buf[32] == '.' && buf[33] == '.') {
			return nil // the folder's first cluster holds something else now
		}
		if _, valid := parseDir(buf, 0, true); valid == 0 && c != first {
			break
		}
		out = append(out, buf...)
	}
	return out
}

func (vol *volume) walk(ctx context.Context, dir *fsys.Entry, raw []byte, base int64, depth int) {
	if depth > 64 || ctx.Err() != nil {
		return
	}
	entries, _ := parseDir(raw, base, true)
	for _, d := range entries {
		e := vol.entry(d)
		if e.IsDir {
			if d.cluster == 0 || !vol.b.validCluster(d.cluster) {
				continue
			}
			if _, seen := vol.dirs[d.cluster]; seen {
				continue // a loop, or a deleted copy of a folder already read
			}
			dir.Add(e)
			vol.dirs[d.cluster] = e
			var sub []byte
			if d.deleted || dir.State == fsys.Deleted {
				sub = vol.readDeletedDir(d.cluster)
			} else {
				sub = vol.readChain(d.cluster)
			}
			vol.walk(ctx, e, sub, vol.b.clusterOffset(d.cluster), depth+1)
			continue
		}
		dir.Add(e)
	}
}
