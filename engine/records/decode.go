package records

import (
	"bytes"
	"encoding/binary"
)

func parseInt(s string) (int64, bool) {
	var n int64
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int64(r-'0')
	}
	return n, true
}

// typedText reads the text out of an NSAttributedString archived as a
// typedstream, as iOS keeps a message's in attributedBody: the C string
// after the class name NSString.
func typedText(b []byte) string {
	i := bytes.Index(b, []byte("NSString"))
	if i < 0 {
		return ""
	}
	b = b[i+len("NSString"):]
	j := bytes.IndexByte(b, '+')
	if j < 0 || j > 8 || j+2 > len(b) {
		return ""
	}
	b = b[j+1:]
	n := int(b[0])
	b = b[1:]
	switch {
	case n == 0x81 && len(b) >= 2:
		n = int(binary.LittleEndian.Uint16(b))
		b = b[2:]
	case n == 0x82 && len(b) >= 4:
		n = int(binary.LittleEndian.Uint32(b))
		b = b[4:]
	}
	return string(b[:min(n, len(b))])
}
