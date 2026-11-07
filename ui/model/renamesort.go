package model

import (
	"os"
	"sort"
	"strings"
)

func sortFiles(files []string, o RenameOrder) {
	switch o {
	case OrderByModified:
		mod := map[string]int64{}
		for _, f := range files {
			if fi, err := os.Stat(f); err == nil {
				mod[f] = fi.ModTime().UnixNano()
			}
		}
		sort.SliceStable(files, func(i, j int) bool { return mod[files[i]] < mod[files[j]] })
	default:
		sort.Slice(files, func(i, j int) bool { return naturalLess(files[i], files[j]) })
	}
}

// naturalLess orders names as people do: "IMG 2" before "IMG 10".
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func digits(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}
