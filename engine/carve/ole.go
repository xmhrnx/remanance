package carve

import (
	"io"
	"strings"
	"unicode/utf16"
)

// parseOLE sizes a compound file (DOC, XLS, PPT, MSG, older MSI): its
// allocation table says which sectors are in use, and the file ends after
// the last one.
func parseOLE(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 512, limit)
	if h == nil || le16(h[28:]) != 0xFFFE {
		return Match{}
	}
	shift := le16(h[30:])
	if shift != 9 && shift != 12 {
		return Match{}
	}
	ss := int64(1) << shift
	fatSectors := int64(le32(h[44:]))
	dirStart := int64(le32(h[48:]))
	if fatSectors == 0 || fatSectors > 1<<20 {
		return Match{}
	}
	sectorOff := func(n int64) int64 { return (n + 1) * ss }
	var fats []int64
	for i := 0; i < 109 && int64(len(fats)) < fatSectors; i++ {
		fats = append(fats, int64(le32(h[76+4*i:])))
	}
	// More FAT sector numbers continue in DIFAT sectors.
	for d, hops := int64(le32(h[68:])), 0; d < 0xFFFFFFFA && int64(len(fats)) < fatSectors && hops < 4096; hops++ {
		b := at(r, sectorOff(d), int(ss), limit)
		if b == nil {
			break
		}
		for i := int64(0); i < ss/4-1 && int64(len(fats)) < fatSectors; i++ {
			fats = append(fats, int64(le32(b[4*i:])))
		}
		d = int64(le32(b[ss-4:]))
	}
	last := int64(-1)
	for i, f := range fats {
		b := at(r, sectorOff(f), int(ss), limit)
		if b == nil {
			if last < 0 {
				return Match{}
			}
			return Match{Length: sectorOff(last + 1), Partial: true}
		}
		for j := int64(0); j < ss/4; j++ {
			if le32(b[4*j:]) != 0xFFFFFFFF {
				last = int64(i)*(ss/4) + j
			}
		}
	}
	if last < 0 {
		return Match{}
	}
	length := sectorOff(last + 1)
	if length > limit {
		return Match{Length: limit, Partial: true, Ext: oleKind(r, sectorOff(dirStart), ss, limit)}
	}
	return Match{Length: length, Ext: oleKind(r, sectorOff(dirStart), ss, limit)}
}

// oleKind names the document type by the streams in the first directory
// sector.
func oleKind(r io.ReaderAt, dirOff, ss, limit int64) string {
	b := at(r, dirOff, int(ss), limit)
	if b == nil {
		return "ole"
	}
	for i := int64(0); i+128 <= ss; i += 128 {
		n := int(le16(b[i+64:]))
		if n < 2 || n > 64 {
			continue
		}
		u := make([]uint16, n/2-1)
		for k := range u {
			u[k] = le16(b[i+int64(2*k):])
		}
		name := string(utf16.Decode(u))
		switch {
		case name == "WordDocument":
			return "doc"
		case name == "Workbook" || name == "Book":
			return "xls"
		case name == "PowerPoint Document":
			return "ppt"
		case strings.HasPrefix(name, "__substg1.0_") || name == "__properties_version1.0":
			return "msg"
		case name == "VisioDocument":
			return "vsd"
		}
	}
	return "ole"
}

func init() {
	Register(&Signature{
		Name: "Office 97-2003 document", Ext: "doc", Category: Documents, MaxSize: 2 * GB,
		Magics: []Magic{{0, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}}},
		Parse:  parseOLE,
	})
}
