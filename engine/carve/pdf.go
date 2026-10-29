package carve

import (
	"bytes"
	"io"
)

// parsePDF looks for the last "%%EOF" that still belongs to the file: a
// PDF edited in place appends updates, each with its own "%%EOF", so the
// file ends at the last one not followed by more PDF objects.
func parsePDF(r io.ReaderAt, limit int64) Match {
	const chunk = 1 << 20
	buf := make([]byte, chunk+8)
	end := int64(0)
	for off := int64(0); off < limit; off += chunk {
		n, _ := r.ReadAt(buf[:min(int64(len(buf)), limit-off)], off)
		if n == 0 {
			break
		}
		data := buf[:n]
		for i := 0; ; {
			j := bytes.Index(data[i:], []byte("%%EOF"))
			if j < 0 {
				break
			}
			pos := off + int64(i+j+5)
			// Include the line ending after the marker.
			if e := at(r, pos, 2, limit); e != nil {
				switch {
				case e[0] == '\r' && e[1] == '\n':
					pos += 2
				case e[0] == '\n' || e[0] == '\r':
					pos++
				}
			}
			end = pos
			i += j + 5
		}
		if end > 0 && off+int64(n) > end+64*KB && !continuesPDF(r, end, limit) {
			return Match{Length: end}
		}
		if n < len(buf) {
			break
		}
	}
	if end == 0 {
		return Match{}
	}
	return Match{Length: end}
}

// continuesPDF peeks after an "%%EOF" for an incremental update.
func continuesPDF(r io.ReaderAt, off, limit int64) bool {
	b := at(r, off, int(min(4096, limit-off)), limit)
	if b == nil {
		return false
	}
	b = bytes.TrimLeft(b, "\r\n\t \x00")
	if len(b) == 0 {
		return false
	}
	head := b[:min(len(b), 64)]
	return bytes.Contains(head, []byte(" obj")) || bytes.HasPrefix(b, []byte("xref")) || bytes.HasPrefix(b, []byte("%"))
}

func init() {
	Register(&Signature{
		Name: "PDF document", Ext: "pdf", Category: Documents, MaxSize: 2 * GB,
		Magics: []Magic{{0, []byte("%PDF-")}},
		Parse:  parsePDF,
	})
}
