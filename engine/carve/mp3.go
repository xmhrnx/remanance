package carve

import "io"

var (
	mp3Bitrates = [2][3][16]int{
		{ // MPEG 1: layers I, II, III
			{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448, 0},
			{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0},
			{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
		},
		{ // MPEG 2 and 2.5
			{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0},
			{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
			{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
		},
	}
	mp3Rates = [4][3]int{{11025, 12000, 8000}, {0, 0, 0}, {22050, 24000, 16000}, {44100, 48000, 32000}}
)

// mp3Frame returns the length of the MPEG audio frame whose header is h.
func mp3Frame(h []byte) int64 {
	if h[0] != 0xFF || h[1]&0xE0 != 0xE0 {
		return 0
	}
	version := h[1] >> 3 & 3 // 0: 2.5, 2: 2, 3: 1
	layer := h[1] >> 1 & 3   // 1: III, 2: II, 3: I
	br := h[2] >> 4
	sr := h[2] >> 2 & 3
	if version == 1 || layer == 0 || br == 0 || br == 15 || sr == 3 {
		return 0
	}
	v := 1
	if version == 3 {
		v = 0
	}
	l := 3 - int(layer)
	bitrate := mp3Bitrates[v][l][br] * 1000
	rate := mp3Rates[version][sr]
	pad := int64(h[2] >> 1 & 1)
	switch {
	case l == 0:
		return (12*int64(bitrate)/int64(rate) + pad) * 4
	case l == 2 && v == 1:
		return 72*int64(bitrate)/int64(rate) + pad
	}
	return 144*int64(bitrate)/int64(rate) + pad
}

// parseMP3 walks MPEG frames after an optional ID3v2 tag, and takes an
// ID3v1 tag at the end. Without a tag it insists on a long run of
// frames: a stray FF FB is common in other data.
func parseMP3(r io.ReaderAt, limit int64) Match {
	off := int64(0)
	h := at(r, 0, 10, limit)
	if h == nil {
		return Match{}
	}
	tagged := string(h[:3]) == "ID3"
	if tagged {
		if h[3] > 4 || h[6]|h[7]|h[8]|h[9] >= 0x80 {
			return Match{}
		}
		size := int64(h[6])<<21 | int64(h[7])<<14 | int64(h[8])<<7 | int64(h[9])
		off = 10 + size
		if h[5]&0x10 != 0 {
			off += 10
		}
		// Some encoders pad between the tag and the first frame.
		for i := 0; i < 4096; i++ {
			b := at(r, off, 2, limit)
			if b == nil || b[0] == 0xFF && b[1]&0xE0 == 0xE0 {
				break
			}
			off++
		}
	}
	s := newStream(r, limit)
	frames := 0
	hdr := make([]byte, 4)
	for {
		ok := true
		for i := range hdr {
			if hdr[i], ok = s.byteAt(off + int64(i)); !ok {
				break
			}
		}
		if !ok {
			break
		}
		n := mp3Frame(hdr)
		if n < 24 {
			break
		}
		frames++
		off += n
	}
	if frames < 8 && !(tagged && frames > 0) {
		return Match{}
	}
	if t := at(r, off, 3, limit); t != nil && string(t) == "TAG" {
		off += 128
	}
	return Match{Length: min(off, limit)}
}

func init() {
	Register(&Signature{
		Name: "MP3 audio", Ext: "mp3", Category: Audio, MaxSize: 1 * GB,
		Magics: []Magic{{0, []byte("ID3")}, {0, []byte{0xFF, 0xFB}}, {0, []byte{0xFF, 0xF3}}, {0, []byte{0xFF, 0xFA}}, {0, []byte{0xFF, 0xF2}}},
		Parse:  parseMP3,
	})
}
