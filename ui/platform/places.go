package platform

import "os"

// Place is a folder offered as a starting point: the user's own folders,
// and the drives.
type Place struct {
	Label string
	Path  string
	Icon  string
	Drive bool
}

// Places lists the folders and drives worth offering, those there.
func Places() []Place {
	var out []Place
	for _, p := range places() {
		if info, err := os.Stat(p.Path); err == nil && info.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// PicksFolders is true where the system's folder picker gives a path.
const PicksFolders = ChooseDestination
