package rename

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// foldNames is true where the file system ignores case in names.
var foldNames = runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "ios"

func key(dir, name string) string {
	k := filepath.Join(dir, name)
	if foldNames {
		k = strings.ToLower(k)
	}
	return k
}

// check finds what's wrong with each new name.
func check(cs []Change) {
	moving := map[string]bool{} // files this batch renames
	for _, c := range cs {
		if !c.Same() {
			moving[key(filepath.Dir(c.Path), filepath.Base(c.Path))] = true
		}
	}
	taken := map[string]int{}
	for _, c := range cs {
		taken[key(filepath.Dir(c.Path), c.New)]++
	}
	for i := range cs {
		c := &cs[i]
		dir := filepath.Dir(c.Path)
		k := key(dir, c.New)
		switch {
		case c.New == "" || strings.HasPrefix(c.New, "."):
			c.Problem = "The name would be empty"
		case strings.ContainsAny(c.New, `/\`) || strings.ContainsAny(c.New, invalid) || hasControl(c.New):
			c.Problem = "The name has characters files can't have"
		case reserved(c.New):
			c.Problem = "The system reserves that name"
		case len(c.New) > 255:
			c.Problem = "The name is too long"
		case taken[k] > 1:
			c.Problem = "Another file would get the same name"
		case c.Same():
		case !moving[k] && exists(dir, c.New) && !strings.EqualFold(filepath.Base(c.Path), c.New):
			c.Problem = "A file with that name is already there"
		}
	}
}

func exists(dir, name string) bool {
	_, err := os.Lstat(filepath.Join(dir, name))
	return err == nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 32 {
			return true
		}
	}
	return false
}

// invalid are characters Windows doesn't allow in names, refused on
// every system so files can move between them.
const invalid = `<>:"|?*`

// reserved tells whether Windows reserves a name, such as CON or LPT1.
func reserved(name string) bool {
	stem := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9' {
		return true
	}
	return strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ")
}
