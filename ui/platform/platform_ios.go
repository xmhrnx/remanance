//go:build ios

package platform

import (
	"os"
	"path/filepath"

	"github.com/mehran9675/remanence/engine/recovery"
)

const (
	ChooseDestination = false
	CanReveal         = false
)

// DefaultDestination is the app's Documents folder, which the Files app
// shows under On My iPhone › Remanence.
func DefaultDestination() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "Recovered")
}

// DestinationNote says where to find the files.
func DestinationNote() string {
	return "Files are saved in the Files app, under On My iPhone › Remanence › Recovered."
}

// Sink saves into the app's Documents.
func Sink(string) (recovery.Sink, error) {
	dir := DefaultDestination()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return recovery.DirSink{Dir: dir}, nil
}
