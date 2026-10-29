package carve

import "io"

// parseGIF walks the image and extension blocks to the trailer.
func parseGIF(r io.ReaderAt, limit int64) Match {
	s := newStream(r, limit)
	lsd := at(r, 6, 7, limit)
	if lsd == nil {
		return Match{}
	}
	s.pos = 13
	if lsd[4]&0x80 != 0 {
		s.pos += 3 << (lsd[4]&7 + 1)
	}
	images := 0
	skipBlocks := func() bool {
		for {
			n, ok := s.next()
			if !ok {
				return false
			}
			if n == 0 {
				return true
			}
			s.pos += int64(n)
		}
	}
	for {
		b, ok := s.next()
		if !ok {
			break
		}
		switch b {
		case 0x3B:
			if images == 0 {
				return Match{}
			}
			return Match{Length: s.pos}
		case 0x21:
			if _, ok := s.next(); !ok || !skipBlocks() {
				return partialGIF(s.pos, images)
			}
		case 0x2C:
			d := at(r, s.pos, 9, limit)
			if d == nil {
				return partialGIF(s.pos, images)
			}
			s.pos += 9
			if d[8]&0x80 != 0 {
				s.pos += 3 << (d[8]&7 + 1)
			}
			s.pos++ // LZW minimum code size
			if !skipBlocks() {
				return partialGIF(s.pos, images)
			}
			images++
		default:
			return partialGIF(s.pos-1, images)
		}
	}
	return partialGIF(s.pos, images)
}

func partialGIF(pos int64, images int) Match {
	if images == 0 {
		return Match{}
	}
	return Match{Length: pos, Partial: true}
}

func init() {
	Register(&Signature{
		Name: "GIF image", Ext: "gif", Category: Pictures, MaxSize: 128 * MB,
		Magics: []Magic{{0, []byte("GIF87a")}, {0, []byte("GIF89a")}},
		Parse:  parseGIF,
	})
}
