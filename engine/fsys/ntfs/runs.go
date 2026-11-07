package ntfs

import (
	"errors"
	"fmt"
	"io"
	"sort"
)

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
