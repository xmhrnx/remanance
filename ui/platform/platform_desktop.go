//go:build !android && !ios

package platform

import (
	"os"
	"path/filepath"

	"github.com/mehran9675/remanence/engine/recovery"
)

// ChooseDestination is true where the user picks the folder.
const ChooseDestination = true

// CanReveal is true where the file manager can be opened on the result.
const CanReveal = true

// DefaultDestination suggests a folder in the user's home. It is often on
// the system drive, so the dialog checks it against the scanned drive.
func DefaultDestination() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Recovered files")
}

// DestinationNote explains where files go when the user doesn't choose.
func DestinationNote() string { return "" }

// Sink saves into dir.
func Sink(dir string) (recovery.Sink, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return recovery.DirSink{Dir: dir}, nil
}
