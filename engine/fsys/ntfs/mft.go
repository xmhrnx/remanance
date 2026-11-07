package ntfs

import (
	"context"
	"io"

	"github.com/xmhrnx/remanance/engine/fsys"
)

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
