package carve

import (
	"hash/crc32"
	"io"
)

// parsePNG walks the chunks to IEND, checking every chunk's CRC: a
// mismatch means the file is damaged or continues in another fragment.
func parsePNG(r io.ReaderAt, limit int64) Match {
	off := int64(8)
	first := true
	for {
		h := at(r, off, 8, limit)
		if h == nil {
			break
		}
		n := int64(be32(h))
		typ := string(h[4:8])
		if n > 256*MB || !isChunkType(h[4:8]) || first && typ != "IHDR" {
			break
		}
		data := at(r, off+8, int(n)+4, limit)
		if data == nil {
			break
		}
		crc := crc32.NewIEEE()
		crc.Write(h[4:8])
		crc.Write(data[:n])
		if crc.Sum32() != be32(data[n:]) {
			if first {
				return Match{}
			}
			return Match{Length: off, Partial: true}
		}
		first = false
		off += 12 + n
		if typ == "IEND" {
			return Match{Length: off}
		}
	}
	if first {
		return Match{}
	}
	return Match{Length: off, Partial: true}
}

func isChunkType(b []byte) bool {
	for _, c := range b {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}

func init() {
	Register(&Signature{
		Name: "PNG image", Ext: "png", Category: Pictures, MaxSize: 512 * MB,
		Magics: []Magic{{0, []byte("\x89PNG\r\n\x1a\n")}},
		Parse:  parsePNG,
	})
}
