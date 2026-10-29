package carve

import (
	"bytes"
	"io"
)

// ebmlVint reads an EBML variable-length integer. With raw set, the
// length marker stays in (element IDs); otherwise it's stripped (sizes).
func ebmlVint(b []byte, raw bool) (val uint64, n int, unknown bool) {
	if len(b) == 0 || b[0] == 0 {
		return 0, 0, false
	}
	n = 1
	for mask := byte(0x80); b[0]&mask == 0; mask >>= 1 {
		n++
	}
	if n > 8 || n > len(b) {
		return 0, 0, false
	}
	v := uint64(b[0])
	if !raw {
		v &= uint64(0xFF >> n)
	}
	all := v == uint64(0xFF>>n)
	for i := 1; i < n; i++ {
		v = v<<8 | uint64(b[i])
		all = all && b[i] == 0xFF
	}
	return v, n, all && !raw
}

// parseMKV sizes Matroska and WebM files: the EBML header element, then
// the Segment element, whose size covers the rest.
func parseMKV(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 64, limit)
	if h == nil {
		return Match{}
	}
	size, n, _ := ebmlVint(h[4:], false)
	if n == 0 || size > 1024 {
		return Match{}
	}
	head := at(r, 4+int64(n), int(size), limit)
	ext := "mkv"
	if head != nil && bytes.Contains(head, []byte("webm")) {
		ext = "webm"
	}
	off := 4 + int64(n) + int64(size)
	s := at(r, off, 12, limit)
	if s == nil {
		return Match{}
	}
	id, idn, _ := ebmlVint(s, true)
	if id != 0x18538067 {
		return Match{}
	}
	segSize, sn, unknown := ebmlVint(s[idn:], false)
	if sn == 0 {
		return Match{}
	}
	if unknown {
		// Live recordings write an unknown size: the segment runs to
		// wherever the clusters stop.
		return Match{Length: limit, Ext: ext, Partial: true}
	}
	end := off + int64(idn+sn) + int64(segSize)
	if end > limit {
		return Match{Length: limit, Ext: ext, Partial: true}
	}
	return Match{Length: end, Ext: ext}
}

func init() {
	Register(&Signature{
		Name: "Matroska or WebM video", Ext: "mkv", Category: Videos, MaxSize: 64 * GB,
		Magics: []Magic{{0, []byte{0x1A, 0x45, 0xDF, 0xA3}}},
		Parse:  parseMKV,
	})
}
