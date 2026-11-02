//go:build !android && !ios

package platform

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/xmhrnx/remanance/locales"
)

func places() []Place {
	home, _ := os.UserHomeDir()
	out := []Place{{Label: locales.T("places.home"), Path: home, Icon: "folder"}}
	for _, d := range []struct{ name, icon string }{
		{"Desktop", "folder"}, {"Documents", "document"}, {"Downloads", "folder-open"},
		{"Pictures", "picture"}, {"Videos", "video"}, {"Movies", "video"}, {"Music", "music"},
	} {
		out = append(out, Place{Label: d.name, Path: filepath.Join(home, d.name), Icon: d.icon})
	}
	return append(out, drives()...)
}

// drives are the drives mounted on this computer.
func drives() []Place {
	var out []Place
	switch runtime.GOOS {
	case "windows":
		for c := 'C'; c <= 'Z'; c++ {
			root := string(c) + `:\`
			if _, err := os.Stat(root); err == nil {
				out = append(out, Place{Label: locales.T("places.drive", "letter", string(c)), Path: root, Icon: "drive", Drive: true})
			}
		}
	case "darwin":
		out = append(out, Place{Label: "Macintosh HD", Path: "/", Icon: "drive", Drive: true})
		vols, _ := filepath.Glob("/Volumes/*")
		for _, v := range vols {
			if fi, err := os.Lstat(v); err == nil && fi.Mode()&os.ModeSymlink == 0 {
				out = append(out, Place{Label: filepath.Base(v), Path: v, Icon: "usb", Drive: true})
			}
		}
	default:
		out = append(out, Place{Label: locales.T("places.this_computer"), Path: "/", Icon: "drive", Drive: true})
		user := os.Getenv("USER")
		var mounts []string
		for _, pat := range []string{"/media/" + user + "/*", "/run/media/" + user + "/*", "/mnt/*"} {
			m, _ := filepath.Glob(pat)
			mounts = append(mounts, m...)
		}
		for _, m := range mounts {
			out = append(out, Place{Label: filepath.Base(m), Path: m, Icon: "usb", Drive: true})
		}
	}
	return out
}
