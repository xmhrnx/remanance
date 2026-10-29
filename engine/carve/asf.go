package carve

import (
	"bytes"
	"io"
)

var (
	asfHeader = []byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	asfData   = []byte{0x36, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	asfIndex  = [][]byte{
		{0x90, 0x08, 0x00, 0x33, 0xB1, 0xE5, 0xCF, 0x11, 0x89, 0xF4, 0x00, 0xA0, 0xC9, 0x03, 0x49, 0xCB}, // simple index
		{0xD3, 0x29, 0xE2, 0xD6, 0xDA, 0x35, 0xD1, 0x11, 0x90, 0x34, 0x00, 0xA0, 0xC9, 0x03, 0x49, 0xBE}, // index
		{0xF8, 0x03, 0xB1, 0xFE, 0xAD, 0x12, 0x64, 0x4C, 0x84, 0x0F, 0x2A, 0x1D, 0x2F, 0x7A, 0xD4, 0x8C}, // media object index
		{0xD0, 0x3F, 0xB7, 0x3C, 0x4A, 0x0C, 0x03, 0x48, 0x95, 0x3D, 0xED, 0xF7, 0xB6, 0x22, 0x8F, 0x0C}, // timecode index
	}
)

// parseASF sizes Windows Media files (WMV, WMA): the header object, the
// data object and any index objects after it, each with its size.
func parseASF(r io.ReaderAt, limit int64) Match {
	off := int64(0)
	video := false
	for objects := 0; objects < 16; objects++ {
		h := at(r, off, 24, limit)
		if h == nil {
			break
		}
		size := int64(le64(h[16:]))
		if size < 24 {
			break
		}
		switch {
		case objects == 0 && bytes.Equal(h[:16], asfHeader):
			if hdr := at(r, off, int(min(size, 64*KB)), limit); hdr != nil {
				// Video stream type GUID begins BC 19 EF C0.
				video = bytes.Contains(hdr, []byte{0xC0, 0xEF, 0x19, 0xBC})
			}
		case objects == 1 && bytes.Equal(h[:16], asfData):
		case objects >= 2 && isASFIndex(h[:16]):
		default:
			if objects < 2 {
				return Match{}
			}
			return asfMatch(off, limit, video)
		}
		off += size
	}
	return asfMatch(off, limit, video)
}

func isASFIndex(g []byte) bool {
	for _, x := range asfIndex {
		if bytes.Equal(g, x) {
			return true
		}
	}
	return false
}

func asfMatch(end, limit int64, video bool) Match {
	ext := "wma"
	if video {
		ext = "wmv"
	}
	if end > limit {
		return Match{Length: limit, Ext: ext, Partial: true}
	}
	return Match{Length: end, Ext: ext}
}

func init() {
	Register(&Signature{
		Name: "Windows Media file", Ext: "wmv", Category: Videos, MaxSize: 16 * GB,
		Magics: []Magic{{0, asfHeader}},
		Parse:  parseASF,
	})
}
