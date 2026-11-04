//go:build linux && !android

package cleanup

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Sources are the places Linux and its apps keep files that can go.
func Sources() []Source {
	out := []Source{{Kind: Temp, Dir: os.TempDir(), Each: true}, {Kind: Temp, Dir: "/var/tmp", Each: true}}
	if cache, err := os.UserCacheDir(); err == nil {
		out = append(out, Source{Kind: Caches, Dir: cache, Each: true})
	}
	home, err := os.UserHomeDir()
	if err == nil {
		out = append(out, places(BuildCaches, "npm", filepath.Join(home, ".npm", "_cacache"))...)
	}
	out = append(out, Source{Kind: Logs, Dir: "/var/crash", Each: true}, Source{Kind: Trash})
	if err == nil {
		out = append(out, Source{Kind: OldDownloads, Dir: downloads(home), Each: true})
	}
	return out
}

// downloads is the user's Downloads folder, which may have another name
// in their language, as user-dirs.dirs says.
func downloads(home string) string {
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	if f, err := os.Open(filepath.Join(config, "user-dirs.dirs")); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "XDG_DOWNLOAD_DIR="); ok {
				v = strings.ReplaceAll(strings.Trim(v, `"`), "$HOME", home)
				if v != home {
					return v
				}
			}
		}
	}
	return filepath.Join(home, "Downloads")
}

// Drive is the user's home, on the drive cleaning frees space on.
func Drive() string { home, _ := os.UserHomeDir(); return home }
