package disk

import (
	"path/filepath"
	"strings"
)

// VolumeFor finds, among infos, the volume a folder is on, and the
// folder's path inside it with forward slashes ("" for its top).
func VolumeFor(infos []Info, dir string) (Info, string, bool) {
	dir = filepath.Clean(dir)
	var best Info
	var rel string
	longest := -1
	for _, in := range infos {
		if in.Kind != KindVolume {
			continue
		}
		for _, m := range in.Mounts {
			root := m
			if len(m) == 2 && m[1] == ':' {
				root += string(filepath.Separator) // C: alone means the current folder there
			}
			root = filepath.Clean(root)
			r, ok := inside(root, dir)
			if ok && len(root) > longest {
				best, rel, longest = in, r, len(root)
			}
		}
	}
	return best, rel, longest >= 0
}

// inside says whether dir is root or in it, and where.
func inside(root, dir string) (string, bool) {
	r, err := filepath.Rel(root, dir)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) || filepath.IsAbs(r) {
		return "", false
	}
	if r == "." {
		return "", true
	}
	return filepath.ToSlash(r), true
}
