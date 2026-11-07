package partition

import "sort"

// Lost returns the volumes found that the partition table doesn't
// account for, as partitions marked Lost.
func (f *Finder) Lost(table []Partition) []Partition {
	f.mu.Lock()
	cands := make([]*candidate, 0, len(f.found))
	for _, c := range f.found {
		cands = append(cands, c)
	}
	f.mu.Unlock()
	sort.Slice(cands, func(i, j int) bool { return cands[i].Offset < cands[j].Offset })

	var out []Partition
	for _, c := range cands {
		// Seen only once as a backup that couldn't be confirmed: too weak.
		if c.Backup && c.hits < 2 && !c.verified {
			continue
		}
		if listed(table, c.Volume) || insideRecognized(table, c.Volume) || insideFound(out, c.Volume) {
			continue
		}
		length := c.Length
		if c.Offset+length > f.size {
			length = f.size - c.Offset
		}
		out = append(out, Partition{
			Index: len(table) + len(out) + 1, Scheme: SchemeFound, Offset: c.Offset, Length: length,
			Type: "Found " + c.FS, FS: c.FS, Label: c.Label, Lost: true,
		})
	}
	return out
}

func listed(table []Partition, v Volume) bool {
	for _, p := range table {
		if p.Offset == v.Offset {
			return true
		}
	}
	return false
}

// insideRecognized reports a volume nested in a healthy listed partition:
// a disk image or a VM disk stored as a file inside it, not a lost volume.
func insideRecognized(table []Partition, v Volume) bool {
	for _, p := range table {
		if p.FS != "" && v.Offset > p.Offset && v.Offset < p.End() {
			return true
		}
	}
	return false
}

func insideFound(found []Partition, v Volume) bool {
	for _, p := range found {
		if v.Offset > p.Offset && v.Offset+v.Length <= p.End() {
			return true
		}
	}
	return false
}
