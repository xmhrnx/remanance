package carve

import "io"

func rarVint(b []byte) (uint64, int) {
	var v uint64
	for i := 0; i < len(b) && i < 10; i++ {
		v |= uint64(b[i]&0x7F) << (7 * i)
		if b[i]&0x80 == 0 {
			return v, i + 1
		}
	}
	return 0, 0
}

// parseRAR walks the blocks of RAR 4 and RAR 5 archives to the end of
// archive block.
func parseRAR(r io.ReaderAt, limit int64) Match {
	sig := at(r, 0, 8, limit)
	if sig == nil {
		return Match{}
	}
	if sig[6] == 0x01 && sig[7] == 0x00 {
		return parseRAR5(r, limit)
	}
	off := int64(7)
	for blocks := 0; blocks < 1<<20; blocks++ {
		h := at(r, off, 11, limit)
		if h == nil {
			break
		}
		typ := h[2]
		flags := le16(h[3:])
		size := int64(le16(h[5:]))
		if typ < 0x72 || typ > 0x7B || size < 7 {
			break
		}
		if flags&0x8000 != 0 {
			size += int64(le32(h[7:]))
		}
		off += size
		if typ == 0x7B {
			return Match{Length: off}
		}
	}
	if off > 7 {
		return Match{Length: min(off, limit), Partial: true}
	}
	return Match{}
}

func parseRAR5(r io.ReaderAt, limit int64) Match {
	off := int64(8)
	for blocks := 0; blocks < 1<<20; blocks++ {
		h := at(r, off, 32, limit)
		if h == nil {
			break
		}
		hsize, n1 := rarVint(h[4:])
		if n1 == 0 || hsize == 0 || hsize > 2*MB {
			break
		}
		body := h[4+n1:]
		typ, n2 := rarVint(body)
		flags, n3 := rarVint(body[n2:])
		if n2 == 0 || n3 == 0 || typ < 1 || typ > 5 {
			break
		}
		p := n2 + n3
		if flags&1 != 0 {
			_, n := rarVint(body[p:])
			p += n
		}
		var data uint64
		if flags&2 != 0 {
			data, _ = rarVint(body[p:])
		}
		off += 4 + int64(n1) + int64(hsize) + int64(data)
		if typ == 5 { // end of archive
			return Match{Length: off}
		}
	}
	if off > 8 {
		return Match{Length: min(off, limit), Partial: true}
	}
	return Match{}
}

func init() {
	Register(&Signature{
		Name: "RAR archive", Ext: "rar", Category: Archives, MaxSize: 64 * GB,
		Magics: []Magic{{0, []byte("Rar!\x1A\x07")}},
		Parse:  parseRAR,
	})
}
