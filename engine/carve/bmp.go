package carve

import "io"

func parseBMP(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 54, limit)
	if h == nil {
		return Match{}
	}
	size := int64(le32(h[2:]))
	pixels := int64(le32(h[10:]))
	dib := le32(h[14:])
	if le32(h[6:]) != 0 || size < 54 || pixels >= size || pixels < 26 {
		return Match{}
	}
	switch dib {
	case 12, 40, 52, 56, 64, 108, 124:
	default:
		return Match{}
	}
	if dib >= 40 {
		w, hgt := int32(le32(h[18:])), int32(le32(h[22:]))
		bpp := le16(h[28:])
		if w <= 0 || w > 1<<16 || hgt == 0 || bpp == 0 || bpp > 32 {
			return Match{}
		}
	}
	if size > limit {
		return Match{Length: limit, Partial: true}
	}
	return Match{Length: size}
}

func init() {
	Register(&Signature{
		Name: "Bitmap image", Ext: "bmp", Category: Pictures, MaxSize: 1 * GB,
		Magics: []Magic{{0, []byte("BM")}},
		Parse:  parseBMP,
	})
}
