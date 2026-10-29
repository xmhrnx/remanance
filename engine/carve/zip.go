package carve

import (
	"bytes"
	"io"
	"strings"
)

// parseZIP finds the end of central directory record whose directory
// offset and size point back to the start: that pins the archive's exact
// length and rules out an EOCD belonging to another zip. The central
// directory's names tell Office documents, ODF, EPUB, APK and JAR apart.
func parseZIP(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 30, limit)
	if h == nil || le16(h[4:]) > 63 {
		return Match{}
	}
	switch le16(h[8:]) { // compression methods in use
	case 0, 1, 6, 8, 9, 12, 14, 93, 95, 98, 99:
	default:
		return Match{}
	}
	const chunk = 1 << 20
	buf := make([]byte, chunk+22)
	for off := int64(0); off < limit; off += chunk {
		n, _ := r.ReadAt(buf[:min(int64(len(buf)), limit-off)], off)
		if n < 22 {
			break
		}
		data := buf[:n]
		for i := 0; ; {
			j := bytes.Index(data[i:], []byte("PK\x05\x06"))
			if j < 0 || i+j+22 > len(data) {
				break
			}
			e := data[i+j:]
			pos := off + int64(i+j)
			cdSize, cdOff := int64(le32(e[12:])), int64(le32(e[16:]))
			comment := int64(le16(e[20:]))
			if cdOff+cdSize == pos || cdOff == 0xFFFFFFFF {
				if cdOff == 0xFFFFFFFF {
					cdOff, cdSize = zip64Directory(r, pos, limit)
				}
				return Match{Length: pos + 22 + comment, Ext: zipKind(r, cdOff, cdSize, limit)}
			}
			i += j + 4
		}
		if n < len(buf) {
			break
		}
	}
	return Match{}
}

// zip64Directory reads the ZIP64 end record that precedes the locator
// just before the EOCD.
func zip64Directory(r io.ReaderAt, eocd, limit int64) (int64, int64) {
	loc := at(r, eocd-20, 20, limit)
	if loc == nil || string(loc[:4]) != "PK\x06\x07" {
		return 0, 0
	}
	rec := at(r, int64(le64(loc[8:])), 56, limit)
	if rec == nil || string(rec[:4]) != "PK\x06\x06" {
		return 0, 0
	}
	return int64(le64(rec[48:])), int64(le64(rec[40:]))
}

func zipKind(r io.ReaderAt, cdOff, cdSize, limit int64) string {
	if cdSize <= 0 || cdSize > 64*MB {
		return "zip"
	}
	cd := at(r, cdOff, int(cdSize), limit)
	if cd == nil {
		return "zip"
	}
	var names []string
	for p := 0; p+46 <= len(cd) && string(cd[p:p+4]) == "PK\x01\x02" && len(names) < 4000; {
		nl, el, cl := int(le16(cd[p+28:])), int(le16(cd[p+30:])), int(le16(cd[p+32:]))
		if p+46+nl > len(cd) {
			break
		}
		names = append(names, string(cd[p+46:p+46+nl]))
		p += 46 + nl + el + cl
	}
	has := func(prefix string) bool {
		for _, n := range names {
			if strings.HasPrefix(n, prefix) {
				return true
			}
		}
		return false
	}
	switch {
	case has("word/"):
		return "docx"
	case has("xl/"):
		return "xlsx"
	case has("ppt/"):
		return "pptx"
	case has("AndroidManifest.xml"):
		return "apk"
	case has("META-INF/container.xml") && has("mimetype"):
		return "epub"
	case has("content.xml") && has("mimetype"):
		if lh := at(r, 30, 8, limit); lh != nil && string(lh) == "mimetype" {
			if mt := at(r, 38, 50, limit); mt != nil {
				switch {
				case bytes.Contains(mt, []byte("spreadsheet")):
					return "ods"
				case bytes.Contains(mt, []byte("presentation")):
					return "odp"
				}
			}
		}
		return "odt"
	case has("META-INF/MANIFEST.MF"):
		return "jar"
	}
	return "zip"
}

func zipCategory(ext string) Category {
	switch ext {
	case "docx", "xlsx", "pptx", "odt", "ods", "odp", "epub":
		return Documents
	case "apk", "jar":
		return Other
	}
	return Archives
}

func init() {
	Register(&Signature{
		Name: "ZIP archive or Office document", Ext: "zip", Category: Archives, MaxSize: 8 * GB,
		Magics: []Magic{{0, []byte("PK\x03\x04")}},
		Parse:  parseZIP,
	})
}
