package carve

import "io"

// parseRIFF handles WAV, AVI and WebP: a RIFF header with the total size
// and a form type, then chunks.
func parseRIFF(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 20, limit)
	if h == nil {
		return Match{}
	}
	size := int64(le32(h[4:])) + 8
	var ext string
	switch string(h[8:12]) {
	case "WAVE":
		ext = "wav"
	case "AVI ":
		ext = "avi"
	case "WEBP":
		ext = "webp"
	case "RMID":
		ext = "rmi"
	case "CDXA":
		ext = "dat"
	default:
		return Match{}
	}
	if !isFourCC(h[12:16]) || size < 20 {
		return Match{}
	}
	// AVIs over 1 GB continue in "AVIX" RIFF lists after the first.
	if ext == "avi" {
		for next := size; next+12 <= limit; {
			x := at(r, next, 12, limit)
			if x == nil || string(x[:4]) != "RIFF" || string(x[8:12]) != "AVIX" {
				break
			}
			next += int64(le32(x[4:])) + 8
			size = next
		}
	}
	if size > limit {
		return Match{Length: limit, Ext: ext, Partial: true}
	}
	return Match{Length: size, Ext: ext}
}

func isFourCC(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}

// parseAIFF handles the big-endian IFF cousin, AIFF.
func parseAIFF(r io.ReaderAt, limit int64) Match {
	h := at(r, 0, 16, limit)
	if h == nil || (string(h[8:12]) != "AIFF" && string(h[8:12]) != "AIFC") || !isFourCC(h[12:16]) {
		return Match{}
	}
	size := int64(be32(h[4:])) + 8
	if size > limit {
		return Match{Length: limit, Partial: true}
	}
	return Match{Length: size}
}

func init() {
	Register(&Signature{
		Name: "RIFF media (WAV, AVI, WebP)", Ext: "wav", Category: Audio, MaxSize: 16 * GB,
		Magics: []Magic{{0, []byte("RIFF")}},
		Parse:  parseRIFF,
	})
	Register(&Signature{
		Name: "AIFF audio", Ext: "aiff", Category: Audio, MaxSize: 4 * GB,
		Magics: []Magic{{0, []byte("FORM")}},
		Parse:  parseAIFF,
	})
}
