package rename

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// expand fills in a pattern's fields.
func expand(pattern, p, stem, ext string, n int) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(pattern, '{')
		if i < 0 {
			b.WriteString(pattern)
			return b.String()
		}
		j := strings.IndexByte(pattern[i:], '}')
		if j < 0 {
			b.WriteString(pattern)
			return b.String()
		}
		b.WriteString(pattern[:i])
		field, arg, _ := strings.Cut(pattern[i+1:i+j], ":")
		switch field {
		case "name":
			b.WriteString(stem)
		case "ext":
			b.WriteString(ext)
		case "n":
			s := strconv.Itoa(n)
			if w, err := strconv.Atoi(arg); err == nil && len(s) < w {
				s = strings.Repeat("0", w-len(s)) + s
			}
			b.WriteString(s)
		case "date":
			if arg == "" {
				arg = "2006-01-02"
			}
			b.WriteString(dateOf(p).Format(arg))
		case "parent":
			b.WriteString(filepath.Base(filepath.Dir(p)))
		default:
			b.WriteString(pattern[i : i+j+1])
		}
		pattern = pattern[i+j+1:]
	}
}

// dateOf is when a photo was taken, or when the file last changed.
func dateOf(p string) time.Time {
	if t, ok := photoDate(p); ok {
		return t
	}
	if info, err := os.Stat(p); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func changeCase(s string, c Case) string {
	switch c {
	case CaseLower:
		return strings.ToLower(s)
	case CaseUpper:
		return strings.ToUpper(s)
	case CaseTitle, CaseSentence:
		rs := []rune(strings.ToLower(s))
		start := true
		for i, r := range rs {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				if start {
					rs[i] = unicode.ToUpper(r)
					start = false
				}
				continue
			}
			if c == CaseTitle && (r == ' ' || r == '_' || r == '-' || r == '.') {
				start = true
			}
		}
		return string(rs)
	}
	return s
}
