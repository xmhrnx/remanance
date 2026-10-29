package recovery

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

var reserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// SafeName makes a recovered name valid on every platform: names come
// from damaged metadata and other systems, and may hold anything.
func SafeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7F || unicode.Is(unicode.Cs, r) || r == unicode.ReplacementChar:
			b.WriteRune('_')
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	s := strings.TrimRight(b.String(), ". ")
	if s == "" {
		s = "unnamed"
	}
	base := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	if reserved[base] {
		s = "_" + s
	}
	// Leave room in a 255-byte name limit, keeping the extension.
	if len(s) > 200 {
		ext := path.Ext(s)
		if len(ext) > 16 {
			ext = ""
		}
		cut := 200 - len(ext)
		for cut > 0 && !utf8Start(s[cut]) {
			cut--
		}
		s = s[:cut] + ext
	}
	return s
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// unique returns rel, or "name (2).ext" and so on when rel is taken.
func unique(s Sink, rel string) string {
	if !s.Exists(rel) {
		return rel
	}
	dir, file := path.Split(rel)
	ext := path.Ext(file)
	stem := strings.TrimSuffix(file, ext)
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s%s (%d)%s", dir, stem, i, ext)
		if !s.Exists(c) {
			return c
		}
	}
}
