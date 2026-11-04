//go:build unix && !darwin && !android && !ios

package trash

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const supported = true

// move moves the file into the freedesktop.org trash in the user's data
// folder, with the .trashinfo file the file managers read to put it
// back.
func move(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	trash, err := userTrash()
	if err != nil {
		return err
	}
	files, info := filepath.Join(trash, "files"), filepath.Join(trash, "info")
	if err := os.MkdirAll(files, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(info, 0o700); err != nil {
		return err
	}
	// Claim a name by creating its info file.
	base := filepath.Base(abs)
	ext := filepath.Ext(base)
	var name string
	var f *os.File
	for i := 0; ; i++ {
		name = base
		if i > 0 {
			name = fmt.Sprintf("%s.%d%s", strings.TrimSuffix(base, ext), i, ext)
		}
		f, err = os.OpenFile(filepath.Join(info, name+".trashinfo"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) || i > 1000 {
			return err
		}
	}
	u := (&url.URL{Path: abs}).EscapedPath()
	fmt.Fprintf(f, "[Trash Info]\nPath=%s\nDeletionDate=%s\n", u, time.Now().Format("2006-01-02T15:04:05"))
	f.Close()
	if err := os.Rename(abs, filepath.Join(files, name)); err != nil {
		os.Remove(filepath.Join(info, name+".trashinfo"))
		if errors.Is(err, syscall.EXDEV) {
			return ErrNoTrash
		}
		return err
	}
	return nil
}

// userTrash is the trash in the user's data folder.
func userTrash() (string, error) {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		data = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(data, "Trash"), nil
}

func usage() (Usage, error) {
	dir, err := userTrash()
	if err != nil {
		return Usage{}, err
	}
	return dirUsage(filepath.Join(dir, "files"))
}

// empty deletes the trashed files and the notes saying where they came
// from.
func empty() error {
	dir, err := userTrash()
	if err != nil {
		return err
	}
	if err := clearDir(filepath.Join(dir, "files")); err != nil {
		return err
	}
	os.Remove(filepath.Join(dir, "directorysizes"))
	return clearDir(filepath.Join(dir, "info"))
}
