package ntfs

import (
	"fmt"
	"sort"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// fill copies a node's metadata and data location into its entry.
func (vol *volume) fill(n *node) *fsys.Entry {
	rec := n.rec
	e := &fsys.Entry{
		ID:       uint64(rec.number),
		Name:     n.name.name,
		IsDir:    rec.isDir(),
		Created:  n.si.created,
		Modified: n.si.modified,
		Accessed: n.si.accessed,
		Health:   100,
	}
	if e.Modified.IsZero() {
		e.Created, e.Modified, e.Accessed = n.name.created, n.name.modified, n.name.accessed
	}
	if !rec.inUse() {
		e.State = fsys.Deleted
	}
	if rec.torn {
		e.Note = "MFT record partly overwritten"
	}
	if e.IsDir || len(n.data) == 0 {
		return e
	}
	sort.Slice(n.data, func(i, j int) bool { return n.data[i].startVCN < n.data[j].startVCN })
	first := n.data[0]
	if first.resident {
		e.Size = int64(len(first.value))
		e.Inline = append([]byte(nil), first.value...)
		return e
	}
	e.Size = first.dataSize
	cs := vol.b.clusterSize
	for _, r := range joinRuns(n.data) {
		off := int64(-1)
		if r.lcn >= 0 {
			off = vol.devOff + r.lcn*cs
		}
		e.Runs = append(e.Runs, fsys.Run{Offset: off, Length: r.length * cs})
	}
	switch {
	case first.flags&flagEncrypted != 0:
		e.Note = "Encrypted with EFS: the copy stays encrypted"
	case first.flags&flagCompressed != 0 && first.compUnitLog > 0:
		e.Codec = "lznt1"
		e.Unit = cs << first.compUnitLog
	}
	if e.State == fsys.Deleted && vol.bitmap != nil {
		e.Health = fsys.Health(e.Runs, cs, vol.devOff, vol.bitmap.Used)
		if e.Health < 100 && e.Note == "" {
			e.Note = "Some clusters have been reused since deletion"
		}
	}
	return e
}

// build turns the records into a folder tree.
func (vol *volume) build(nodes []*node) {
	for i, n := range nodes {
		if n == nil || !n.named {
			continue
		}
		if i < firstUser && i != recordRoot {
			continue
		}
		n.entry = vol.fill(n)
	}
	if recordRoot < len(nodes) && nodes[recordRoot] != nil && nodes[recordRoot].entry != nil {
		root := nodes[recordRoot].entry
		vol.v.Root.ID = root.ID
		vol.v.Root.Created, vol.v.Root.Modified = root.Created, root.Modified
		nodes[recordRoot].entry = vol.v.Root
	} else {
		vol.v.Warn("the root folder's record is missing; files are placed by their folders' record numbers")
	}

	lostDirs := map[int64]*fsys.Entry{}
	lostDir := func(parent int64) *fsys.Entry {
		if d, ok := lostDirs[parent]; ok {
			return d
		}
		d := &fsys.Entry{ID: uint64(parent), Name: fmt.Sprintf("Folder %d", parent), IsDir: true, State: fsys.Orphan, Health: 100}
		vol.v.Lost.Add(d)
		lostDirs[parent] = d
		return d
	}

	for i, n := range nodes {
		if n == nil || n.entry == nil || i == recordRoot {
			continue
		}
		e := n.entry
		if hideSystem(n, nodes) {
			continue
		}
		p := n.name.parent
		var parent *node
		if p >= 0 && p < int64(len(nodes)) {
			parent = nodes[p]
		}
		switch {
		case parent != nil && parent.entry != nil && parent.entry.IsDir && ownsChild(parent, n) && !createsCycle(parent.entry, e):
			parent.entry.Add(e)
		default:
			if e.State == fsys.Existing {
				e.State = fsys.Orphan
			}
			lostDir(p).Add(e)
		}
	}
}
