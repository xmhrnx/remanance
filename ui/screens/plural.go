package screens

import "github.com/xmhrnx/remanance/locales"

// count says how many of a thing there are, "1 file" or "3 files": noun
// is the key of its message under count. in the locales.
func count(n int, noun string) string { return locales.N("count."+noun, n) }

// label is an engine's English name for something, in the user's
// language when the locales have it under label.
func label(s string) string { return locales.Label(s) }

// labels is label for each of a list.
func labels(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = label(s)
	}
	return out
}
