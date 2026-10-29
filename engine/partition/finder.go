package partition

import (
	"encoding/binary"
	"io"
	"sort"
	"sync"
)

// Finder collects volumes seen during a linear scan of a device and turns
// them into partitions the table doesn't list: deleted partitions, a
// repartitioned drive, a table overwritten by a quick format.
type Finder struct {
	r    io.ReaderAt
	size int64

	mu    sync.Mutex
	found map[int64]*candidate
}

type candidate struct {
	Volume
	hits     int  // copies of its boot record seen (primary, backups)
	verified bool // its metadata was found where the boot record says
}

// NewFinder searches the device r of the given size.
func NewFinder(r io.ReaderAt, size int64) *Finder {
	return &Finder{r: r, size: size, found: map[int64]*candidate{}}
}

// Sector examines one sector-aligned block at device offset pos; buf holds
// at least 1024 bytes from there. It is safe to call from several
// goroutines.
func (f *Finder) Sector(buf []byte, pos int64) {
	// Cheap rejections first: this runs for every sector of the device.
	b3 := buf[3]
	extMagic := len(buf) >= 58 && buf[56] == 0x53 && buf[57] == 0xEF
	hfs := buf[0] == 'H' && (buf[1] == '+' || buf[1] == 'X')
	nx := len(buf) >= 36 && buf[32] == 'N' && buf[33] == 'X'
	boot := buf[510] == 0x55 && buf[511] == 0xAA && (buf[0] == 0xEB || buf[0] == 0xE9 || b3 == 'N' || b3 == 'E')
	if !boot && !extMagic && !hfs && !nx {
		return
	}
	var v Volume
	var ok bool
	switch {
	case boot:
		v, ok = probeBoot(buf[:512], pos)
		if ok {
			v = f.placeBoot(buf[:512], v)
		}
	case extMagic:
		v, ok = probeExt(buf[:1024], pos)
	case hfs:
		v, ok = probeHFS(buf[:512], pos)
	case nx:
		v, ok = probeAPFS(buf, pos)
	}
	if !ok || v.Offset < 0 || v.Length <= 0 || v.Offset >= f.size {
		return
	}
	f.add(v)
}

// placeBoot decides whether a boot sector is a volume's primary copy or a
// backup, by looking for the metadata each reading predicts.
func (f *Finder) placeBoot(s []byte, v Volume) Volume {
	bps := int64(binary.LittleEndian.Uint16(s[11:]))
	switch v.FS {
	case "NTFS":
		spc := int64(s[13])
		if spc > 0x80 {
			spc = 1 << (256 - spc)
		}
		mft := int64(binary.LittleEndian.Uint64(s[0x30:])) * spc * bps
		if f.hasFileRecord(v.Offset + mft) {
			return v
		}
		// The backup sits in the volume's last sector.
		start := v.Offset - (v.Length - bps)
		if start >= 0 && f.hasFileRecord(start+mft) {
			v.Offset, v.Backup = start, true
		}
	case "exFAT":
		// The backup boot region begins 12 sectors in.
		if !f.sameBoot(v.Offset-12*bps, s) {
			return v
		}
		v.Offset -= 12 * bps
		v.Backup = true
	case "FAT32":
		backup := int64(binary.LittleEndian.Uint16(s[50:]))
		if backup > 0 && backup < 32 && f.sameBoot(v.Offset-backup*bps, s) {
			v.Offset -= backup * bps
			v.Backup = true
		}
	}
	return v
}

func (f *Finder) hasFileRecord(off int64) bool {
	if off < 0 || off+4 > f.size {
		return false
	}
	b := make([]byte, 4)
	if _, err := f.r.ReadAt(b, off); err != nil {
		return false
	}
	return string(b) == "FILE"
}

func (f *Finder) sameBoot(off int64, s []byte) bool {
	if off < 0 {
		return false
	}
	b := make([]byte, 512)
	if _, err := f.r.ReadAt(b, off); err != nil {
		return false
	}
	return string(b[3:11]) == string(s[3:11]) && string(b[11:90]) == string(s[11:90])
}

func (f *Finder) add(v Volume) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.found[v.Offset]
	if c == nil {
		c = &candidate{Volume: v}
		f.found[v.Offset] = c
	}
	c.hits++
	if !v.Backup && c.Backup {
		c.Volume = v // prefer what the primary copy says
	}
	if v.FS == "NTFS" || !v.Backup {
		c.verified = c.verified || f.verify(v)
	}
}

func (f *Finder) verify(v Volume) bool {
	got, ok := Probe(f.r, v.Offset)
	return ok && got.FS == v.FS
}

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
