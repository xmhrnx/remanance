package carve

import "io"

// parsePSD sizes Photoshop documents (PSD and large-document PSB): three
// length-prefixed sections, then the merged image, raw or RLE-packed.
func parsePSD(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 26, limit)
	if h == nil {
		return Match{}
	}
	version := be16(h[4:])
	channels := int64(be16(h[12:]))
	height := int64(be32(h[14:]))
	width := int64(be32(h[18:]))
	depth := int64(be16(h[22:]))
	if version != 1 && version != 2 || channels == 0 || channels > 56 || width == 0 || height == 0 || depth == 0 {
		return Match{}
	}
	off := int64(26)
	for section := 0; section < 3; section++ {
		w := 4
		if version == 2 && section == 2 {
			w = 8
		}
		b := at(r, off, w, limit)
		if b == nil {
			return Match{}
		}
		n := int64(be32(b))
		if w == 8 {
			n = int64(be64(b))
		}
		off += int64(w) + n
	}
	c := at(r, off, 2, limit)
	if c == nil {
		return Match{Length: min(off, limit), Partial: true}
	}
	off += 2
	rows := channels * height
	switch be16(c) {
	case 0:
		off += rows * width * depth / 8
	case 1:
		w := int64(2)
		if version == 2 {
			w = 4
		}
		counts := at(r, off, int(rows*w), limit)
		if counts == nil {
			return Match{Length: limit, Partial: true}
		}
		off += rows * w
		for i := int64(0); i < rows; i++ {
			if w == 2 {
				off += int64(be16(counts[i*2:]))
			} else {
				off += int64(be32(counts[i*4:]))
			}
		}
	default:
		return Match{Length: min(off, limit), Partial: true}
	}
	if off > limit {
		return Match{Length: limit, Partial: true}
	}
	return Match{Length: off}
}

func init() {
	Register(&Signature{
		Name: "Photoshop document", Ext: "psd", Category: Pictures, MaxSize: 8 * GB,
		Magics: []Magic{{0, []byte("8BPS")}},
		Parse:  parsePSD,
	})
}
