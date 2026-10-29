package carve

import (
	"bytes"
	"io"
)

// parseOgg walks Ogg pages until the end-of-stream page.
func parseOgg(r io.ReaderAt, limit int64) Match {
	off := int64(0)
	ext := "ogg"
	for pages := 0; ; pages++ {
		h := at(r, off, 27, limit)
		if h == nil || string(h[:4]) != "OggS" || h[4] != 0 {
			if pages == 0 {
				return Match{}
			}
			return Match{Length: off, Ext: ext, Partial: true}
		}
		segs := int(h[26])
		table := at(r, off+27, segs, limit)
		if table == nil {
			return Match{Length: off, Ext: ext, Partial: pages > 0}
		}
		body := int64(0)
		for _, s := range table {
			body += int64(s)
		}
		if pages == 0 {
			if p := at(r, off+27+int64(segs), 8, limit); p != nil && bytes.HasPrefix(p, []byte("OpusHead")) {
				ext = "opus"
			}
		}
		off += 27 + int64(segs) + body
		if h[5]&0x04 != 0 { // end of stream
			return Match{Length: off, Ext: ext}
		}
	}
}

func init() {
	Register(&Signature{
		Name: "Ogg audio", Ext: "ogg", Category: Audio, MaxSize: 2 * GB,
		Magics: []Magic{{0, []byte("OggS\x00\x02")}},
		Parse:  parseOgg,
	})
}
