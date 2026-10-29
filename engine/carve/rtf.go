package carve

import "io"

// parseRTF follows brace nesting to the group that closes the document,
// skipping escaped braces.
func parseRTF(r io.ReaderAt, limit int64) Match {
	s := newStream(r, limit)
	depth := 0
	for {
		c, ok := s.next()
		if !ok {
			return Match{}
		}
		switch c {
		case '\\':
			s.next()
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return Match{Length: s.pos}
			}
		case 0:
			return Match{} // RTF is 7-bit text
		}
	}
}

func init() {
	Register(&Signature{
		Name: "Rich Text document", Ext: "rtf", Category: Documents, MaxSize: 512 * MB,
		Magics: []Magic{{0, []byte(`{\rtf1`)}},
		Parse:  parseRTF,
	})
}
