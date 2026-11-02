//go:build ios

package platform

import (
	"os"
	"path/filepath"
)

// places is the app's own Documents folder: iOS lets an app see no
// other folders by path. Files put there in the Files app show up here.
func places() []Place {
	home, _ := os.UserHomeDir()
	return []Place{{Label: "DataDoc in Files", Path: filepath.Join(home, "Documents"), Icon: "folder", Drive: true}}
}
