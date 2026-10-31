package iphone

import "testing"

func TestOriginal(t *testing.T) {
	for p, want := range map[string]string{
		"PhotoData/Thumbnails/V2/DCIM/100APPLE/IMG_0001.HEIC/5005.JPG": "DCIM/100APPLE/IMG_0001.HEIC",
		"PhotoData/Metadata/DCIM/100APPLE/IMG_0002.JPG":                "DCIM/100APPLE/IMG_0002.JPG",
		"PhotoData/Thumbnails/3303.ithmb":                              "",
		"PhotoData/Thumbnails/V2/DCIM/100APPLE/notes.txt/5005.JPG":     "",
	} {
		if got := original(p); got != want {
			t.Errorf("original(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestParseInfo(t *testing.T) {
	i := parseInfo([]byte("st_size\x001234\x00st_ifmt\x00S_IFDIR\x00st_mtime\x001700000000000000000\x00"))
	if i.size != 1234 || !i.dir || i.link || i.mod.Unix() != 1700000000 {
		t.Errorf("parseInfo = %+v", i)
	}
}
