//go:build android

package platform

import (
	"os"
	"path/filepath"

	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
)

const (
	ChooseDestination = false
	CanReveal         = false
	// ReadsBackups is false: phones keep no backups of other phones.
	ReadsBackups = false
)

// downloads is the phone's shared Downloads folder. Since Android 11 an
// app may create files there by path; Android 10 needs
// requestLegacyExternalStorage, which giode.toml sets.
const downloads = "/storage/emulated/0/Download"

// DefaultDestination is a folder in Downloads.
func DefaultDestination() string { return filepath.Join(downloads, "DataDoc") }

// DestinationNote says where to find the files.
func DestinationNote() string {
	return locales.T("platform.files_are_saved_in_downloads_datadoc")
}

// Sink saves into Downloads/DataDoc, whatever dir says.
func Sink(string) (recovery.Sink, error) {
	dir := DefaultDestination()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return recovery.DirSink{Dir: dir}, nil
}
