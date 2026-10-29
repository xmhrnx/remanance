package carve

import "io"

// parsePE sizes Windows programs and libraries: the end of the last
// section's raw data, or of the signature appended after it.
func parsePE(r io.ReaderAt, limit int64) Match {
	mz := at(r, 0, 64, limit)
	if mz == nil {
		return Match{}
	}
	pe := int64(le32(mz[60:]))
	if pe < 64 || pe > 4096 {
		return Match{}
	}
	h := at(r, pe, 24, limit)
	if h == nil || string(h[:4]) != "PE\x00\x00" {
		return Match{}
	}
	sections := int64(le16(h[6:]))
	optSize := int64(le16(h[20:]))
	if sections == 0 || sections > 96 {
		return Match{}
	}
	opt := at(r, pe+24, int(optSize), limit)
	if opt == nil {
		return Match{}
	}
	ext := "exe"
	if le16(h[22:])&0x2000 != 0 {
		ext = "dll"
	}
	table := at(r, pe+24+optSize, int(sections*40), limit)
	if table == nil {
		return Match{}
	}
	end := pe + 24 + optSize + sections*40
	for i := int64(0); i < sections; i++ {
		s := table[i*40:]
		size, ptr := int64(le32(s[16:])), int64(le32(s[20:]))
		if ptr > 0 && size > 0 {
			end = max(end, ptr+size)
		}
	}
	// The certificate table (data directory 4) is addressed by file
	// offset and usually comes last.
	dirAt := map[uint16]int{0x10B: 96, 0x20B: 112}[le16(opt)]
	if dirAt > 0 && int(optSize) >= dirAt+5*8 {
		off, size := int64(le32(opt[dirAt+32:])), int64(le32(opt[dirAt+36:]))
		if off > 0 && size > 0 {
			end = max(end, off+size)
		}
	}
	if end > limit {
		return Match{Length: limit, Ext: ext, Partial: true}
	}
	return Match{Length: end, Ext: ext}
}

func init() {
	Register(&Signature{
		Name: "Windows program", Ext: "exe", Category: Other, MaxSize: 2 * GB,
		Magics: []Magic{{0, []byte("MZ")}},
		Parse:  parsePE,
	})
}
