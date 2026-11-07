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
