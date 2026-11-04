//go:build android

package cleanup

import (
	"os"
	"path/filepath"
)

// storage is the phone's shared storage.
const storage = "/storage/emulated/0"

// Sources are the places on a phone's shared storage that hold files
// that can go. Other apps' own caches are out of an app's reach.
func Sources() []Source {
	out := []Source{{Kind: Temp, Dir: os.TempDir(), Each: true}}
	out = append(out, places(Caches, "Photo thumbnails", filepath.Join(storage, "DCIM", ".thumbnails"), filepath.Join(storage, "Pictures", ".thumbnails"))...)
	return append(out, Source{Kind: OldDownloads, Dir: filepath.Join(storage, "Download"), Each: true})
}

// Drive is the phone's shared storage.
func Drive() string { return storage }
