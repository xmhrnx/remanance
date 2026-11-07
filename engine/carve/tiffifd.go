package carve

import "strings"

// walk reads the IFD at off and the IFDs it links to.
func (t *tiffFile) walk(off int64, depth int) {
	for hops := 0; off >= 8 && off < t.limit && hops < 64 && depth < 4; hops++ {
		cnt := at(t.r, off, 2, t.limit)
		if cnt == nil {
			return
		}
		n := int64(t.order.Uint16(cnt))
		if n == 0 || n > 1000 {
			return
		}
		ents := at(t.r, off+2, int(n*12+4), t.limit)
		if ents == nil {
			return
		}
		t.ifds++
		t.end = max(t.end, off+2+n*12+4)
		var stripOff, stripLen, tileOff, tileLen []int64
		var jpegOff, jpegLen int64
		for i := int64(0); i < n; i++ {
			e := ents[i*12:]
			tag := t.order.Uint16(e)
			typ := t.order.Uint16(e[2:])
			count := int64(t.order.Uint32(e[4:]))
			if typ == 0 || int(typ) >= len(tiffTypeSize) || count < 0 || count > 1<<24 {
				continue
			}
			size := tiffTypeSize[typ] * count
			valOff := int64(-1)
			if size > 4 {
				valOff = int64(t.order.Uint32(e[8:]))
				if valOff+size <= t.limit {
					t.end = max(t.end, valOff+size)
				}
			}
			switch tag {
			case 0x010F:
				t.make = t.ascii(e, valOff, size)
			case 0x0110:
				t.model = t.ascii(e, valOff, size)
			case 0x9003, 0x0132:
				if d := t.ascii(e, valOff, size); d != "" && (t.date == "" || tag == 0x9003) {
					t.date = d
				}
			case 0xC612:
				t.dng = true
			case 0x0111:
				stripOff = t.values(e, typ, count, valOff)
			case 0x0117:
				stripLen = t.values(e, typ, count, valOff)
			case 0x0144:
				tileOff = t.values(e, typ, count, valOff)
			case 0x0145:
				tileLen = t.values(e, typ, count, valOff)
			case 0x0201:
				jpegOff = t.first(e, typ)
			case 0x0202:
				jpegLen = t.first(e, typ)
			case 0x8769, 0x8825, 0xA005: // EXIF, GPS, interoperability IFDs
				t.walk(t.first(e, typ), depth+1)
			case 0x014A: // SubIFDs: raw files keep the full image here
				for _, o := range t.values(e, typ, count, valOff) {
					t.walk(o, depth+1)
				}
			}
		}
		t.extend(stripOff, stripLen)
		t.extend(tileOff, tileLen)
		if jpegOff > 0 && jpegLen > 0 && jpegOff+jpegLen <= t.limit {
			t.end = max(t.end, jpegOff+jpegLen)
		}
		off = int64(t.order.Uint32(ents[n*12:]))
	}
}

func (t *tiffFile) extend(offs, lens []int64) {
	for i := 0; i < len(offs) && i < len(lens); i++ {
		if e := offs[i] + lens[i]; offs[i] > 0 && e <= t.limit {
			t.end = max(t.end, e)
		}
	}
}

func (t *tiffFile) first(e []byte, typ uint16) int64 {
	if typ == 3 {
		return int64(t.order.Uint16(e[8:]))
	}
	return int64(t.order.Uint32(e[8:]))
}

func (t *tiffFile) values(e []byte, typ uint16, count, valOff int64) []int64 {
	if typ != 3 && typ != 4 && typ != 13 || count > 1<<16 {
		return nil
	}
	w := int64(4)
	if typ == 3 {
		w = 2
	}
	data := e[8:12]
	if valOff >= 0 {
		data = at(t.r, valOff, int(count*w), t.limit)
		if data == nil {
			return nil
		}
	}
	out := make([]int64, 0, count)
	for i := int64(0); i < count && (i+1)*w <= int64(len(data)); i++ {
		if w == 2 {
			out = append(out, int64(t.order.Uint16(data[i*2:])))
		} else {
			out = append(out, int64(t.order.Uint32(data[i*4:])))
		}
	}
	return out
}

func (t *tiffFile) ascii(e []byte, valOff, size int64) string {
	var b []byte
	if valOff < 0 {
		b = e[8 : 8+size]
	} else if size < 256 {
		b = at(t.r, valOff, int(size), t.limit)
	}
	return strings.TrimSpace(strings.TrimRight(string(b), "\x00"))
}
