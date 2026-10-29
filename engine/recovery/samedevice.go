package recovery

import (
	"path/filepath"
	"strings"

	"github.com/mehran9675/remanence/engine/disk"
)

// SameDevice reports whether dest lies on the device being recovered.
// Writing there can overwrite the very data being recovered, so the app
// refuses it.
func SameDevice(info disk.Info, dest string) bool {
	if info.Kind == disk.KindImage {
		// An image's own file must not be overwritten, but its folder is
		// fine.
		return false
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		return false
	}
	for _, m := range info.Mounts {
		if m == "" {
			continue
		}
		if len(m) == 2 && m[1] == ':' { // a Windows drive letter
			if strings.EqualFold(filepath.VolumeName(abs), m) {
				return true
			}
			continue
		}
		if m == "/" {
			// The root mount is on this device; anything not on another
			// mount is too. Without a mount table here, err on the side of
			// refusing.
			return true
		}
		if abs == m || strings.HasPrefix(abs, strings.TrimSuffix(m, "/")+"/") {
			return true
		}
	}
	return false
}
