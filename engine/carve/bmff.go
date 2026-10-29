package carve

import (
	"io"
	"strings"
)

// The ISO base media file format: MP4, MOV, M4A, 3GP, HEIC, AVIF, CR3.
// A file is a sequence of top-level boxes, each starting with its size.

var topBoxes = map[string]bool{
	"ftyp": true, "moov": true, "mdat": true, "free": true, "skip": true, "wide": true,
	"uuid": true, "meta": true, "pnot": true, "pdin": true, "moof": true, "mfra": true,
	"styp": true, "sidx": true, "ssix": true, "emsg": true, "prft": true, "junk": true,
	"PICT": true, "beam": true, "idat": true, "iinf": true, "iloc": true, "pict": true,
}

func parseBMFF(r io.ReaderAt, limit int64) Match {
	off := int64(0)
	var brand string
	var sawMovie, sawMedia, sawFtyp bool
	for boxes := 0; boxes < 10000; boxes++ {
		h := at(r, off, 16, limit)
		if h == nil {
			break
		}
		size := int64(be32(h))
		typ := string(h[4:8])
		if !topBoxes[typ] {
			if boxes == 0 {
				return Match{}
			}
			break // the next bytes aren't a box: the file ended here
		}
		switch size {
		case 0: // the box runs to the end of the file
			if sawMovie || sawFtyp {
				return Match{Length: limit, Ext: bmffExt(brand, typ), Partial: true}
			}
			return Match{}
		case 1:
			size = int64(be64(h[8:]))
			if size < 16 {
				return Match{}
			}
		default:
			if size < 8 {
				break
			}
		}
		if size < 8 {
			break
		}
		switch typ {
		case "ftyp":
			if boxes != 0 && !sawMovie {
				break
			}
			sawFtyp = true
			brand = string(h[8:12])
		case "moov", "meta":
			sawMovie = true
		case "mdat":
			sawMedia = true
		}
		if off+size > limit {
			if sawFtyp {
				return Match{Length: limit, Ext: bmffExt(brand, typ), Partial: true}
			}
			return Match{}
		}
		off += size
	}
	if !sawMovie || off == 0 {
		// A camera that died mid-recording leaves mdat without moov: the
		// video data is there but needs repair to play.
		if sawFtyp && sawMedia {
			return Match{Length: off, Ext: bmffExt(brand, ""), Partial: true}
		}
		return Match{}
	}
	return Match{Length: off, Ext: bmffExt(brand, "")}
}

func bmffExt(brand, _ string) string {
	b := strings.TrimSpace(brand)
	switch {
	case b == "heic" || b == "heix" || b == "mif1" || b == "msf1" || b == "heim" || b == "heis":
		return "heic"
	case b == "avif" || b == "avis":
		return "avif"
	case b == "qt":
		return "mov"
	case b == "M4A" || b == "M4B" || b == "M4P":
		return "m4a"
	case strings.HasPrefix(b, "3g"):
		return "3gp"
	case b == "crx":
		return "cr3"
	}
	return "mp4"
}

func bmffCategory(ext string) Category {
	switch ext {
	case "heic", "avif", "cr3":
		return Pictures
	case "m4a":
		return Audio
	}
	return Videos
}

func init() {
	Register(&Signature{
		Name: "MP4, MOV and HEIC media", Ext: "mp4", Category: Videos, MaxSize: 64 * GB,
		Magics: []Magic{{4, []byte("ftyp")}, {4, []byte("moov")}, {4, []byte("mdat")}, {4, []byte("wide")}},
		Parse:  parseBMFF,
	})
}
