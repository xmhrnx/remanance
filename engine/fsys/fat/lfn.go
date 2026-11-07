package fat

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

func lfnChecksum(short []byte) byte {
	var sum byte
	for _, c := range short[:11] {
		sum = (sum&1)<<7 + sum>>1 + c
	}
	return sum
}

// shortNameFirst returns the bytes Windows could have put first in the
// 8.3 name it made for a long name.
func shortNameFirst(long string) []byte {
	s := strings.TrimLeft(strings.ReplaceAll(long, " ", ""), ".")
	if s == "" {
		return []byte{'_'}
	}
	c := s[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	if c < 0x80 && strings.IndexByte("+,;=[]", c) < 0 {
		return []byte{c, '_'}
	}
	return []byte{'_'}
}

func parseLFN(e []byte) lfnPart {
	p := lfnPart{seq: int(e[0] & 0x1F), checksum: e[13]}
	for _, r := range [][2]int{{1, 11}, {14, 26}, {28, 32}} {
		for i := r[0]; i < r[1]; i += 2 {
			p.chars = append(p.chars, binary.LittleEndian.Uint16(e[i:]))
		}
	}
	return p
}

func assembleLFN(parts []lfnPart) string {
	// Parts precede the short entry in reverse order: the last part
	// first.
	var u []uint16
	for i := len(parts) - 1; i >= 0; i-- {
		u = append(u, parts[i].chars...)
	}
	for i, c := range u {
		if c == 0 || c == 0xFFFF {
			u = u[:i]
			break
		}
	}
	return string(utf16.Decode(u))
}
