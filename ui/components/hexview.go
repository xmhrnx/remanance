package components

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/ui/theme"
)

// HexView dumps data eight bytes to a line, which fits a narrow panel:
// offset, bytes, then the bytes as text.
func HexView(data []byte) giode.Element {
	var b strings.Builder
	for off := 0; off < len(data); off += 8 {
		line := data[off:min(off+8, len(data))]
		fmt.Fprintf(&b, "%04X  ", off)
		for i := range 8 {
			if i < len(line) {
				fmt.Fprintf(&b, "%02X ", line[i])
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" ")
		for _, c := range line {
			if c >= 0x20 && c < 0x7f {
				b.WriteByte(c)
			} else {
				b.WriteString("·")
			}
		}
		b.WriteByte('\n')
	}
	return giode.Text(strings.TrimRight(b.String(), "\n"),
		theme.Code().With(giode.Styles{FontSize: 11, LineHeight: 17, Color: giode.Mix("theme.text", "theme.surface", 0.7)})).
		Selectable("hexview")
}

// LooksLikeText reports whether data is readable as text: valid UTF-8
// with few control characters.
func LooksLikeText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// A cut-off rune at the end doesn't count against it.
	for len(data) > 0 && !utf8.Valid(data) {
		data = data[:len(data)-1]
	}
	if len(data) == 0 {
		return false
	}
	bad := 0
	for _, c := range data {
		if c < 0x20 && c != '\n' && c != '\r' && c != '\t' {
			bad++
		}
	}
	return bad*50 < len(data)
}
