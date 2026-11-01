// Package trash moves files to the system's trash, where the user can
// put them back: the Recycle Bin on Windows, the Trash on macOS and the
// freedesktop.org trash on Linux. Phones have none for apps to use.
package trash

import (
	"errors"
	"os"
)

// ErrNoTrash means the file can't go to the trash: the system has none,
// or the file is on a drive the trash doesn't cover.
var ErrNoTrash = errors.New("this file can't go to the trash")

// Supported is true where files can go to the trash.
const Supported = supported

// Move moves the file or folder at path to the trash.
func Move(path string) error {
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	return move(path)
}

// Remove moves path to the trash where there is one, or deletes it.
func Remove(path string, toTrash bool) error {
	if toTrash && Supported {
		return Move(path)
	}
	return os.RemoveAll(path)
}
