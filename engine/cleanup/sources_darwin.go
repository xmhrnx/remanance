//go:build darwin && !ios

package cleanup

import (
	"os"
	"path/filepath"
)

// Sources are the places macOS and its apps keep files that can go.
func Sources() []Source {
	out := []Source{{Kind: Temp, Dir: os.TempDir(), Each: true}}
	home, err := os.UserHomeDir()
	if err != nil {
		return out
	}
	lib := filepath.Join(home, "Library")
	out = append(out, Source{Kind: Caches, Dir: filepath.Join(lib, "Caches"), Each: true})
	out = append(out, places(BuildCaches, "Xcode build data", filepath.Join(lib, "Developer", "Xcode", "DerivedData"))...)
	return append(out, Source{Kind: Logs, Dir: filepath.Join(lib, "Logs"), Each: true},
		Source{Kind: Trash},
		Source{Kind: OldDownloads, Dir: filepath.Join(home, "Downloads"), Each: true})
}

// Drive is the user's home, on the drive cleaning frees space on.
func Drive() string { home, _ := os.UserHomeDir(); return home }
