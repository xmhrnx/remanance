package carve

import (
	"compress/flate"
	"hash/crc32"
	"io"
)

// byteCounter feeds a decompressor one byte at a time (it implements
// io.ByteReader, so flate reads no further than it needs) and counts.
type byteCounter struct {
	s *stream
}

func (b *byteCounter) ReadByte() (byte, error) {
	c, ok := b.s.next()
	if !ok {
		return 0, io.EOF
	}
	return c, nil
}

func (b *byteCounter) Read(p []byte) (int, error) {
	for i := range p {
		c, err := b.ReadByte()
		if err != nil {
			return i, err
		}
		p[i] = c
	}
	return len(p), nil
}

// parseGzip decompresses the stream to find exactly where it ends, and
// checks the CRC in the trailer.
func parseGzip(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 10, limit)
	if h == nil || h[3]&0xE0 != 0 {
		return Match{}
	}
	s := newStream(r, limit)
	s.pos = 10
	flags := h[3]
	if flags&0x04 != 0 { // FEXTRA
		x := at(r, s.pos, 2, limit)
		if x == nil {
			return Match{}
		}
		s.pos += 2 + int64(le16(x))
	}
	for _, f := range []byte{0x08, 0x10} { // FNAME, FCOMMENT: zero-terminated
		if flags&f != 0 {
			for {
				c, ok := s.next()
				if !ok {
					return Match{}
				}
				if c == 0 {
					break
				}
			}
		}
	}
	if flags&0x02 != 0 {
		s.pos += 2
	}
	crc := crc32.NewIEEE()
	fr := flate.NewReader(&byteCounter{s: s})
	n, err := io.Copy(crc, fr)
	if err != nil {
		if n > 0 {
			return Match{Length: s.pos, Partial: true}
		}
		return Match{}
	}
	t := at(r, s.pos, 8, limit)
	if t == nil {
		return Match{Length: s.pos, Partial: true}
	}
	if le32(t) != crc.Sum32() || int64(le32(t[4:])) != n&0xFFFFFFFF {
		return Match{Length: s.pos + 8, Partial: true}
	}
	return Match{Length: s.pos + 8}
}

func init() {
	Register(&Signature{
		Name: "Gzip archive", Ext: "gz", Category: Archives, MaxSize: 4 * GB,
		Magics: []Magic{{0, []byte{0x1F, 0x8B, 0x08}}},
		Parse:  parseGzip,
	})
}
