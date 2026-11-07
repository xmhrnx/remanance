package fat

import "github.com/xmhrnx/remanance/engine/fsys"

// entry turns a directory entry into a tree entry and places its data.
func (vol *volume) entry(d dirent) *fsys.Entry {
	e := &fsys.Entry{
		ID:       uint64(d.offset),
		Name:     d.name,
		IsDir:    d.isDir,
		Created:  d.created,
		Modified: d.modified,
		Accessed: d.accessed,
		Health:   100,
	}
	if d.deleted {
		e.State = fsys.Deleted
	}
	if d.isDir || d.size == 0 {
		return e
	}
	e.Size = d.size
	if !vol.b.validCluster(d.cluster) {
		e.Health = 0
		e.Note = "The first cluster is unknown"
		return e
	}
	cs := vol.b.clusterSize
	need := (d.size + cs - 1) / cs
	var clusters []uint32
	if d.deleted {
		// The chain was cleared: files are usually written contiguously.
		for c := d.cluster; int64(len(clusters)) < need && vol.b.validCluster(c); c++ {
			clusters = append(clusters, c)
		}
	} else {
		clusters = vol.t.chain(d.cluster, need)
		if int64(len(clusters)) < need {
			e.Note = "The cluster chain is shorter than the file"
		}
	}
	e.Runs = vol.runs(clusters)
	if d.deleted {
		e.Health = fsys.Health(e.Runs, cs, vol.devOff+vol.b.dataOffset-2*cs, func(c int64) bool { return vol.t.used(uint32(c)) })
		if e.Health < 100 {
			e.Note = "Some clusters were reused or the file was fragmented"
		}
	}
	return e
}

// runs merges consecutive clusters into device runs.
func (vol *volume) runs(clusters []uint32) []fsys.Run {
	cs := vol.b.clusterSize
	var out []fsys.Run
	for i := 0; i < len(clusters); {
		j := i + 1
		for j < len(clusters) && clusters[j] == clusters[j-1]+1 {
			j++
		}
		out = append(out, fsys.Run{Offset: vol.devOff + vol.b.clusterOffset(clusters[i]), Length: int64(j-i) * cs})
		i = j
	}
	return out
}
