package carve

import "io"

// parseJPEG follows a JPEG's markers to its end-of-image marker, reading
// the entropy-coded scan data byte by byte in between.
func parseJPEG(r io.ReaderAt, limit int64) Match {
	s := newStream(r, limit)
	s.pos = 2
	var sawFrame, sawScan bool
	var name string
	for {
		b, ok := s.next()
		if !ok {
			break
		}
		if b != 0xFF {
			return partialJPEG(s.pos, sawScan, name)
		}
		m, ok := s.next()
		for ok && m == 0xFF { // fill bytes
			m, ok = s.next()
		}
		if !ok {
			break
		}
		switch {
		case m == 0xD9:
			if !sawFrame || !sawScan {
				return Match{}
			}
			return Match{Length: s.pos, Name: name}
		case m >= 0xD0 && m <= 0xD7 || m == 0x01:
			continue
		case m == 0x00 || m == 0xD8:
			return partialJPEG(s.pos, sawScan, name)
		}
		hi, ok1 := s.next()
		lo, ok2 := s.next()
		if !ok1 || !ok2 {
			break
		}
		segLen := int64(hi)<<8 | int64(lo)
		if segLen < 2 {
			return partialJPEG(s.pos, sawScan, name)
		}
		segStart := s.pos
		if m == 0xE1 && name == "" && segLen > 16 {
			name = exifName(r, segStart, segLen-2, limit)
		}
		if m >= 0xC0 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC {
			sawFrame = true
		}
		s.pos = segStart + segLen - 2
		if m != 0xDA {
			continue
		}
		sawScan = true
		// Entropy-coded data: runs until a 0xFF that isn't stuffing
		// (FF 00) or a restart marker.
		for {
			c, ok := s.next()
			if !ok {
				return partialJPEG(s.pos, true, name)
			}
			if c != 0xFF {
				continue
			}
			d, ok := s.byteAt(s.pos)
			if !ok {
				return partialJPEG(s.pos, true, name)
			}
			if d == 0x00 || d >= 0xD0 && d <= 0xD7 || d == 0xFF {
				s.pos++
				continue
			}
			s.pos-- // back onto the 0xFF of the next marker
			break
		}
	}
	return partialJPEG(s.pos, sawScan, name)
}

// partialJPEG reports a JPEG whose end is missing: overwritten or
// fragmented. A start with scan data still shows the top of the picture.
func partialJPEG(pos int64, sawScan bool, name string) Match {
	if !sawScan || pos < 4*KB {
		return Match{}
	}
	return Match{Length: pos, Name: name, Partial: true}
}

func exifName(r io.ReaderAt, off, n, limit int64) string {
	h := at(r, off, 6, limit)
	if h == nil || string(h) != "Exif\x00\x00" {
		return ""
	}
	sub := io.NewSectionReader(r, off+6, n-6)
	t, first, ok := newTIFF(sub, n-6)
	if !ok {
		return ""
	}
	t.walk(first, 0)
	return photoName(t.date)
}

func init() {
	Register(&Signature{
		Name: "JPEG image", Ext: "jpg", Category: Pictures, MaxSize: 128 * MB,
		Magics: []Magic{{0, []byte{0xFF, 0xD8, 0xFF}}},
		Parse:  parseJPEG,
	})
}
