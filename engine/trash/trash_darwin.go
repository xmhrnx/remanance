//go:build darwin && !ios

package trash

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const supported = true

// move moves the file into the user's Trash, or the Trash of the drive
// it's on, under a name that's free there.
func move(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dirs := []string{filepath.Join(home, ".Trash")}
	if v, ok := strings.CutPrefix(abs, "/Volumes/"); ok {
		if i := strings.IndexByte(v, '/'); i > 0 {
			dirs = append(dirs, filepath.Join("/Volumes", v[:i], ".Trashes", strconv.Itoa(os.Getuid())))
		}
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			continue
		}
		err := os.Rename(abs, free(dir, filepath.Base(abs)))
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EXDEV) {
			return err
		}
	}
	return ErrNoTrash
}

// free is a name for base in dir that nothing has yet.
func free(dir, base string) string {
	p := filepath.Join(dir, base)
	if _, err := os.Lstat(p); err != nil {
		return p
	}
	ext := filepath.Ext(base)
	return filepath.Join(dir, strings.TrimSuffix(base, ext)+" "+time.Now().Format("15.04.05.000")+ext)
}

// userTrash is the Trash in the user's home.
func userTrash() (string, error) {
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".Trash"), err
}

// usage adds up the user's Trash. Reading it needs Full Disk Access.
func usage() (Usage, error) {
	dir, err := userTrash()
	if err != nil {
		return Usage{}, err
	}
	return dirUsage(dir)
}

func empty() error {
	dir, err := userTrash()
	if err != nil {
		return err
	}
	return clearDir(dir)
}
