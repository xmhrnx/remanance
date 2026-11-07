// Package iosbackup reads the backups Finder and iTunes make of iPhones
// and iPads. iOS gives no app raw access to a phone's storage, and
// deletes a file's key along with it, so a backup is where an iPhone's
// files can be recovered from: photos, including those in Recently
// Deleted, and every app's files.
package iosbackup

import (
	"errors"
	"sync"
	"time"
)

// Backup describes one backup.
type Backup struct {
	Dir        string // the folder named after the device's identifier
	DeviceName string // "Sam's iPhone"
	Product    string // "iPhone 15 Pro"
	IOS        string // the iOS version backed up
	Date       time.Time
	Encrypted  bool
}

// ErrPassword means the backup is encrypted, and opening it needs the
// password it was made with (see SetPassword).
var ErrPassword = errors.New("this backup is encrypted: enter the password it was made with, the one Finder or iTunes asked for when \"Encrypt local backup\" was turned on")

// ErrWrongPassword means the password set for a backup isn't its own.
var ErrWrongPassword = errors.New("that isn't this backup's password. It's the one set for encrypted backups in Finder or iTunes, not the iPhone's passcode or Apple Account password")

// passwords are the passwords set for encrypted backups, and their
// class keys once unlocked, by backup folder.
var passwords = struct {
	sync.Mutex
	pw   map[string]string
	keys map[string]map[uint32][]byte
}{pw: map[string]string{}, keys: map[string]map[uint32][]byte{}}

// SetPassword sets the password Open tries for the encrypted backup in
// dir.
func SetPassword(dir, password string) {
	passwords.Lock()
	defer passwords.Unlock()
	if passwords.pw[dir] != password {
		delete(passwords.keys, dir)
	}
	passwords.pw[dir] = password
}

// ErrOld means the backup is from before iOS 10, listed in a
// Manifest.mbdb rather than a Manifest.db.
var ErrOld = errors.New("this backup is from iOS 9 or earlier, which DataDoc can't read")

// ErrNotBackup means a folder holds no backup.
var ErrNotBackup = errors.New("that folder isn't an iPhone or iPad backup: choose the folder with Info.plist and Manifest.db in it, or the Backup folder that holds them")
