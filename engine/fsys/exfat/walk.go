package exfat

import (
	"context"

	"github.com/xmhrnx/remanance/engine/fsys"
)

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
