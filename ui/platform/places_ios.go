//go:build ios

package platform

import (
	"os"
	"path/filepath"

	"github.com/xmhrnx/remanance/locales"
)

// places is the app's own Documents folder: iOS lets an app see no
// other folders by path. Files put there in the Files app show up here.
func places() []Place {
	home, _ := os.UserHomeDir()
	return []Place{{Label: locales.T("places.app_in_files"), Path: filepath.Join(home, "Documents"), Icon: "folder", Drive: true}}
}
