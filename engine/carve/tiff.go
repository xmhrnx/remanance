package carve

import (
	"encoding/binary"
	"io"
	"strings"
)

// tiffFile walks a TIFF structure (TIFF images, camera raw files, the
// EXIF block inside JPEGs) to learn its extent and a few tags.
type tiffFile struct {
	r      io.ReaderAt
	limit  int64
	order  binary.ByteOrder
	end    int64 // furthest byte any IFD or its data reaches
	make   string
	model  string
	date   string
	dng    bool
	ifds   int
	strips map[uint16]bool
}

var tiffTypeSize = [...]int64{0, 1, 1, 2, 4, 8, 1, 1, 2, 4, 8, 4, 8, 4}

func newTIFF(r io.ReaderAt, limit int64) (*tiffFile, int64, bool) {
	h := at(r, 0, 8, limit)
	if h == nil {
		return nil, 0, false
	}
	t := &tiffFile{r: r, limit: limit}
	switch string(h[:2]) {
	case "II":
		t.order = binary.LittleEndian
	case "MM":
		t.order = binary.BigEndian
	default:
		return nil, 0, false
	}
	return t, int64(t.order.Uint32(h[4:])), true
}

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

// photoName turns an EXIF date ("2023:05:01 14:22:10") into a file name.
func photoName(date string) string {
	if len(date) < 19 || date[4] != ':' || date[0] == '0' {
		return ""
	}
	return date[0:4] + "-" + date[5:7] + "-" + date[8:10] + " " + date[11:13] + "." + date[14:16] + "." + date[17:19]
}

func parseTIFF(r io.ReaderAt, limit int64) Match {
	t, first, ok := newTIFF(r, limit)
	if !ok {
		return Match{}
	}
	t.walk(first, 0)
	if t.ifds == 0 || t.end <= 8 {
		return Match{}
	}
	ext := "tif"
	mk := strings.ToUpper(t.make)
	hdr := at(r, 0, 12, limit)
	switch {
	case hdr != nil && string(hdr[8:10]) == "CR":
		ext = "cr2"
	case t.dng:
		ext = "dng"
	case strings.HasPrefix(mk, "NIKON"):
		ext = "nef"
	case strings.HasPrefix(mk, "SONY"):
		ext = "arw"
	case strings.HasPrefix(mk, "PENTAX"):
		ext = "pef"
	case strings.HasPrefix(mk, "FUJI"):
		ext = "raf"
	case strings.HasPrefix(mk, "OLYMPUS"):
		ext = "orf"
	case strings.HasPrefix(mk, "PANASONIC"):
		ext = "rw2"
	}
	return Match{Length: t.end, Ext: ext, Name: photoName(t.date)}
}

func init() {
	Register(&Signature{
		Name: "TIFF or camera raw image", Ext: "tif", Category: Pictures, MaxSize: 512 * MB,
		Magics: []Magic{{0, []byte("II*\x00")}, {0, []byte("MM\x00*")}, {0, []byte("IIRO")}, {0, []byte("IIU\x00")}},
		Parse:  parseTIFF,
	})
}
