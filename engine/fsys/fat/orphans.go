package fat

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/mehran9675/remanence/engine/fsys"
)

// findOrphans reads the whole data area for folders the tree doesn't
// reach. A quick format writes a new, empty root and allocation table but
// leaves every subfolder's clusters in place; each still starts with its
// "." and ".." entries, which say where it and its parent were.
func (vol *volume) findOrphans(ctx context.Context, progress fsys.Progress) {
	cs := vol.b.clusterSize
	type orphan struct {
		cluster, parent uint32
		raw             []byte
	}
	var found []orphan
	const batch = 256
	buf := make([]byte, batch*cs)
	total := vol.b.clusters
	for c := uint32(2); c < total+2; c += batch {
		if ctx.Err() != nil {
			return
		}
		n := min(uint32(batch), total+2-c)
		chunk := buf[:int64(n)*cs]
		vol.r.ReadAt(chunk, vol.b.clusterOffset(c))
		for i := uint32(0); i < n; i++ {
			cl := chunk[int64(i)*cs : int64(i+1)*cs]
			if !isDirStart(cl) {
				continue
			}
			self := uint32(binary.LittleEndian.Uint16(cl[20:]))<<16 | uint32(binary.LittleEndian.Uint16(cl[26:]))
			if self != c+i {
				continue // a copy of a folder's start inside some file
			}
			if _, known := vol.dirs[self]; known {
				continue
			}
			parent := uint32(binary.LittleEndian.Uint16(cl[52:]))<<16 | uint32(binary.LittleEndian.Uint16(cl[58:]))
			found = append(found, orphan{cluster: self, parent: parent})
		}
		if progress != nil {
			progress(0.5 + 0.45*float64(c-2)/float64(total))
		}
	}

	// Create every orphan folder first, then nest them by their ".."
	// entries, so a lost "photos/2019" ends up inside a lost "photos".
	byCluster := map[uint32]*fsys.Entry{}
	for _, o := range found {
		e := &fsys.Entry{ID: uint64(o.cluster), Name: fmt.Sprintf("Folder at cluster %d", o.cluster), IsDir: true, State: fsys.Orphan, Health: 100}
		byCluster[o.cluster] = e
		vol.dirs[o.cluster] = e
	}
	for _, o := range found {
		e := byCluster[o.cluster]
		if p, ok := byCluster[o.parent]; ok && p != e {
			p.Add(e)
		} else {
			vol.v.Lost.Add(e)
		}
	}
	for _, o := range found {
		e := byCluster[o.cluster]
		raw := vol.readDeletedDir(o.cluster)
		entries, _ := parseDir(raw, vol.b.clusterOffset(o.cluster), true)
		for _, d := range entries {
			// The allocation table is new: place every file's data as a
			// deleted file's, contiguously from its first cluster.
			wasDeleted := d.deleted
			d.deleted = true
			child := vol.entry(d)
			if !wasDeleted {
				child.State = fsys.Orphan
			}
			if child.IsDir {
				// Named here: give the orphan folder its real name.
				if sub, ok := byCluster[d.cluster]; ok {
					sub.Name = d.name
					sub.Modified = d.modified
					sub.Created = d.created
					continue
				}
				if _, known := vol.dirs[d.cluster]; known || !vol.b.validCluster(d.cluster) {
					continue
				}
				vol.dirs[d.cluster] = child
				e.Add(child)
				vol.walk(ctx, child, vol.readDeletedDir(d.cluster), vol.b.clusterOffset(d.cluster), 1)
				continue
			}
			e.Add(child)
		}
	}
}

// isDirStart recognizes the first cluster of a folder: a "." entry then a
// ".." entry, both marked as directories.
func isDirStart(b []byte) bool {
	return len(b) >= 64 &&
		string(b[0:11]) == ".          " && b[11]&attrDirectory != 0 &&
		string(b[32:43]) == "..         " && b[43]&attrDirectory != 0
}
