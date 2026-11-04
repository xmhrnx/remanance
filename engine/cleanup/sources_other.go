//go:build (!windows && !linux && !darwin) || ios

package cleanup

import "os"

// Sources are the app's own temporary files and caches: elsewhere is out
// of its reach.
func Sources() []Source {
	out := []Source{{Kind: Temp, Dir: os.TempDir(), Each: true}}
	if cache, err := os.UserCacheDir(); err == nil {
		out = append(out, Source{Kind: Caches, Dir: cache, Each: true})
	}
	return out
}

// Drive is where the app keeps its files.
func Drive() string { return os.TempDir() }
