// Package iosbackup reads the backups Finder and iTunes make of iPhones
// and iPads. iOS gives no app raw access to a phone's storage, and
// deletes a file's key along with it, so a backup is where an iPhone's
// files can be recovered from: photos, including those in Recently
// Deleted, and every app's files.
package iosbackup

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/xmhrnx/remanance/engine/plist"
)

// ErrEncrypted means the backup is encrypted, which this package can't
// read yet.
var ErrEncrypted = errors.New("this backup is encrypted, and Remanence can't read encrypted backups yet. " +
	"In Finder or iTunes, turn off \"Encrypt local backup\" (it asks for the backup's password), back up again, and scan the new backup")

// ErrOld means the backup is from before iOS 10, listed in a
// Manifest.mbdb rather than a Manifest.db.
var ErrOld = errors.New("this backup is from iOS 9 or earlier, which Remanence can't read")

// Backup describes one backup.
type Backup struct {
	Dir        string // the folder named after the device's identifier
	DeviceName string // "Sam's iPhone"
	Product    string // "iPhone 15 Pro"
	IOS        string // the iOS version backed up
	Date       time.Time
	Encrypted  bool
}

// Places are where Finder and iTunes keep backups on this computer.
func Places() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(os.Getenv("APPDATA"), "Apple Computer", "MobileSync", "Backup"),
			// iTunes from the Microsoft Store.
			filepath.Join(home, "Apple", "MobileSync", "Backup"),
		}
	case "darwin":
		// Reading it needs Full Disk Access.
		return []string{filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")}
	}
	return nil
}

// Find lists the backups in Places, newest first.
func Find() []Backup {
	var out []Backup
	for _, place := range Places() {
		dirs, _ := os.ReadDir(place)
		for _, d := range dirs {
			if b, err := Read(filepath.Join(place, d.Name())); err == nil {
				out = append(out, b)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	return out
}

// Read describes the backup in dir, from its Info.plist and
// Manifest.plist.
func Read(dir string) (Backup, error) {
	b := Backup{Dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, "Info.plist"))
	if err != nil {
		return b, err
	}
	v, err := plist.Parse(data)
	if err != nil {
		return b, err
	}
	info := plist.Dict(v)
	b.DeviceName = plist.String(info["Device Name"])
	if b.DeviceName == "" {
		b.DeviceName = plist.String(info["Display Name"])
	}
	b.Product = plist.String(info["Product Name"])
	if b.Product == "" {
		b.Product = plist.String(info["Product Type"])
	}
	b.IOS = plist.String(info["Product Version"])
	b.Date, _ = info["Last Backup Date"].(time.Time)
	if data, err := os.ReadFile(filepath.Join(dir, "Manifest.plist")); err == nil {
		if v, err := plist.Parse(data); err == nil {
			b.Encrypted, _ = plist.Dict(v)["IsEncrypted"].(bool)
		}
	}
	return b, nil
}

// ErrNotBackup means a folder holds no backup.
var ErrNotBackup = errors.New("that folder isn't an iPhone or iPad backup: choose the folder with Info.plist and Manifest.db in it, or the Backup folder that holds them")

// ReadPicked reads the backup in dir or, given a folder of backups such
// as MobileSync/Backup, the newest one in it.
func ReadPicked(dir string) (Backup, error) {
	if b, err := Read(dir); err == nil {
		return b, nil
	}
	var newest Backup
	dirs, _ := os.ReadDir(dir)
	for _, d := range dirs {
		if b, err := Read(filepath.Join(dir, d.Name())); err == nil && (newest.Dir == "" || b.Date.After(newest.Date)) {
			newest = b
		}
	}
	if newest.Dir == "" {
		return newest, ErrNotBackup
	}
	return newest, nil
}
