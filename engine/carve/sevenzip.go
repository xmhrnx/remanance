package carve

import (
	"hash/crc32"
	"io"
)

// parse7z sizes a 7-Zip archive from its start header, which is
// checksummed and says where the closing header is.
func parse7z(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 32, limit)
	if h == nil || crc32.ChecksumIEEE(h[12:32]) != le32(h[8:]) {
		return Match{}
	}
	end := 32 + int64(le64(h[12:])) + int64(le64(h[20:]))
	if end <= 32 {
		return Match{}
	}
	if end > limit {
		return Match{Length: limit, Partial: true}
	}
	return Match{Length: end}
}

func init() {
	Register(&Signature{
		Name: "7-Zip archive", Ext: "7z", Category: Archives, MaxSize: 64 * GB,
		Magics: []Magic{{0, []byte{'7', 'z', 0xBC, 0xAF, 0x27, 0x1C}}},
		Parse:  parse7z,
	})
}
