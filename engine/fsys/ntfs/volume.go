package ntfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

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

func init() { fsys.RegisterParser(Open, "NTFS") }

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

func (vol *volume) readRecordAt(off int64, number int64) (*record, error) {
	b := make([]byte, vol.b.recordSize)
	if _, err := vol.r.ReadAt(b, off); err != nil && err != io.EOF {
		return nil, err
	}
	return parseRecord(b, number)
}

// loadMFTRuns finds where the MFT itself is, from record 0 or, when that
// is damaged, from the $MFTMirr copy.
func (vol *volume) loadMFTRuns() error {
	cs := vol.b.clusterSize
	rec, err := vol.readRecordAt(vol.b.mftLCN*cs, recordMFT)
	if err != nil || len(dataPieces(rec)) == 0 {
		vol.v.Warn("MFT record 0 is damaged; using the $MFTMirr copy")
		rec, err = vol.readRecordAt(vol.b.mftMirrLCN*cs, recordMFT)
		if err != nil {
			return fmt.Errorf("ntfs: both copies of the MFT's own record are unreadable: %w", err)
		}
	}
	pieces := dataPieces(rec)
	if len(pieces) == 0 {
		return errors.New("ntfs: the MFT has no data attribute")
	}
	vol.mftRuns = joinRuns(pieces)
	// A heavily fragmented MFT continues its run list in extension
	// records named by its attribute list; they sit in the part already
	// mapped.
	for _, a := range rec.attrs {
		if a.typ != attrList {
			continue
		}
		list := a.value
		if !a.resident {
			list = vol.readRuns(a.runs, a.dataSize)
		}
		for _, ref := range attrListRefs(list, attrData) {
			if ref == recordMFT {
				continue
			}
			ext, err := vol.readRecordAt(vol.recordOffset(ref), ref)
			if err == nil {
				pieces = append(pieces, dataPieces(ext)...)
			}
		}
		vol.mftRuns = joinRuns(pieces)
	}
	return nil
}

// recordOffset maps an MFT record number to its offset in the volume.
func (vol *volume) recordOffset(n int64) int64 {
	pos := n * vol.b.recordSize
	cs := vol.b.clusterSize
	for _, r := range vol.mftRuns {
		bytes := r.length * cs
		if pos < bytes {
			if r.lcn < 0 {
				return -1
			}
			return r.lcn*cs + pos
		}
		pos -= bytes
	}
	return -1
}

func (vol *volume) readRuns(runs []run, size int64) []byte {
	out := make([]byte, 0, size)
	cs := vol.b.clusterSize
	for _, r := range runs {
		n := r.length * cs
		if int64(len(out))+n > size {
			n = size - int64(len(out))
		}
		if n <= 0 {
			break
		}
		b := make([]byte, n)
		if r.lcn >= 0 {
			vol.r.ReadAt(b, r.lcn*cs)
		}
		out = append(out, b...)
	}
	return out
}

func attrListRefs(list []byte, typ uint32) []int64 {
	var out []int64
	for off := 0; off+26 <= len(list); {
		l := int(list[off+4]) | int(list[off+5])<<8
		if l < 26 {
			break
		}
		t := uint32(list[off]) | uint32(list[off+1])<<8 | uint32(list[off+2])<<16 | uint32(list[off+3])<<24
		if t == typ {
			var ref uint64
			for i := 5; i >= 0; i-- {
				ref = ref<<8 | uint64(list[off+16+i])
			}
			out = append(out, int64(ref))
		}
		off += l
	}
	return out
}

func dataPieces(r *record) []attribute {
	var out []attribute
	for _, a := range r.attrs {
		if a.typ == attrData && a.name == "" {
			out = append(out, a)
		}
	}
	return out
}

// joinRuns orders the pieces of a non-resident attribute by starting VCN
// and concatenates their run lists.
func joinRuns(pieces []attribute) []run {
	sort.Slice(pieces, func(i, j int) bool { return pieces[i].startVCN < pieces[j].startVCN })
	var out []run
	for _, p := range pieces {
		if !p.resident {
			out = append(out, p.runs...)
		}
	}
	return out
}

func (vol *volume) mftSize() int64 {
	var n int64
	for _, r := range vol.mftRuns {
		n += r.length * vol.b.clusterSize
	}
	return n
}

// readMFT reads every record of the MFT, in use or not.
func (vol *volume) readMFT(ctx context.Context, progress fsys.Progress) ([]*node, error) {
	rs := vol.b.recordSize
	cs := vol.b.clusterSize
	total := vol.mftSize() / rs
	nodes := make([]*node, total)
	ext := map[int64][]*record{}

	const chunkRecords = 1024
	buf := make([]byte, chunkRecords*rs)
	number := int64(0)
	for _, r := range vol.mftRuns {
		runBytes := r.length * cs
		for done := int64(0); done < runBytes; {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			n := min(int64(len(buf)), runBytes-done)
			chunk := buf[:n]
			if r.lcn < 0 {
				clear(chunk)
			} else if _, err := vol.r.ReadAt(chunk, r.lcn*cs+done); err != nil && err != io.EOF {
				vol.v.Warn("MFT read error at cluster %d: %v", r.lcn+done/cs, err)
				clear(chunk)
			}
			for i := int64(0); i+rs <= n && number < total; i += rs {
				rec, err := parseRecord(chunk[i:i+rs], number)
				if err == nil {
					if number == recordVolume {
						vol.label(rec)
					}
					// Attribute values point into buf, which the next chunk
					// overwrites: keep copies only of what's needed.
					if rec.baseRef != 0 && rec.baseRef != number {
						rec.detach()
						ext[rec.baseRef] = append(ext[rec.baseRef], rec)
					} else {
						nodes[number] = newNode(rec)
						rec.attrs = nil
					}
				}
				number++
			}
			done += n
			if progress != nil && total > 0 {
				progress(float64(number) / float64(total))
			}
		}
	}
	// Fold extension records into their base: more names, more data runs.
	for base, recs := range ext {
		if base < 0 || base >= int64(len(nodes)) || nodes[base] == nil {
			continue
		}
		bn := nodes[base]
		for _, rec := range recs {
			// An extension record belongs to this base only if their
			// states agree; a reused base doesn't own a stale extension.
			if rec.inUse() != bn.rec.inUse() {
				continue
			}
			bn.absorb(rec)
		}
	}
	return nodes, nil
}

func newNode(rec *record) *node {
	n := &node{rec: rec}
	n.absorb(rec)
	return n
}

func (n *node) absorb(rec *record) {
	for _, a := range rec.attrs {
		switch a.typ {
		case attrStandardInfo:
			if si, ok := parseStandardInfo(a.value); ok && rec == n.rec {
				n.si = si
			}
		case attrFileName:
			fn, ok := parseFileName(a.value)
			if !ok {
				continue
			}
			// Prefer the long (Win32) name over the 8.3 DOS one.
			if !n.named || (n.name.namespace == namespaceDOS && fn.namespace != namespaceDOS) {
				n.name, n.named = fn, true
			}
		case attrData:
			if a.name == "" {
				a.value = append([]byte(nil), a.value...)
				n.data = append(n.data, a)
			}
		}
	}
}

func (vol *volume) loadBitmap(nodes []*node) {
	if recordBitmap >= len(nodes) || nodes[recordBitmap] == nil {
		vol.v.Warn("$Bitmap is missing; recovery chances can't be estimated")
		return
	}
	n := nodes[recordBitmap]
	if len(n.data) == 0 {
		return
	}
	size := n.data[0].dataSize
	for _, p := range n.data {
		if p.startVCN == 0 {
			size = p.dataSize
		}
	}
	if n.data[0].resident {
		vol.bitmap = fsys.Bitmap(n.data[0].value)
		return
	}
	vol.bitmap = fsys.Bitmap(vol.readRuns(joinRuns(n.data), size))
}

func (vol *volume) label(rec *record) {
	for _, a := range rec.attrs {
		if a.typ == 0x60 && a.resident { // $VOLUME_NAME
			vol.v.Label = utf16String(a.value)
		}
	}
}

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

// ownsChild checks the sequence number in a file's parent reference: a
// parent record reused for something else since isn't its folder. A
// deleted folder's sequence number went up by one when it was deleted.
func ownsChild(parent, child *node) bool {
	want := child.name.parentSeq
	have := parent.rec.seq
	if want == 0 || have == want {
		return true
	}
	return !parent.rec.inUse() && have == want+1
}

func createsCycle(parent, child *fsys.Entry) bool {
	for p, depth := parent, 0; p != nil && depth < 4096; p, depth = p.Parent, depth+1 {
		if p == child {
			return true
		}
	}
	return false
}

// hideSystem leaves out NTFS's metadata files ($Extend and what's in it).
func hideSystem(n *node, nodes []*node) bool {
	if n.name.parent == recordExtend || n.rec.number == recordExtend {
		return true
	}
	return n.name.parent == recordRoot && strings.HasPrefix(n.name.name, "$") && n.rec.number < 64
}
