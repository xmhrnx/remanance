package iosbackup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xmhrnx/remanance/engine/plist"
)

// unlock unwraps an encrypted backup's class keys with the password set
// for it, and Manifest.db's key with them.
func unlock(dir string) (map[uint32][]byte, []byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, "Manifest.plist"))
	if err != nil {
		return nil, nil, err
	}
	v, err := plist.Parse(data)
	if err != nil {
		return nil, nil, err
	}
	mp := plist.Dict(v)
	bag, _ := mp["BackupKeyBag"].([]byte)
	mkey, _ := mp["ManifestKey"].([]byte)
	kb, err := parseKeybag(bag)
	if err != nil {
		return nil, nil, err
	}

	passwords.Lock()
	pw, set := passwords.pw[dir]
	classes := passwords.keys[dir]
	passwords.Unlock()
	if !set {
		return nil, nil, ErrPassword
	}
	if classes == nil {
		if classes, err = kb.unlock(pw); err == errWrong {
			return nil, nil, ErrWrongPassword
		} else if err != nil {
			return nil, nil, err
		}
		passwords.Lock()
		passwords.keys[dir] = classes
		passwords.Unlock()
	}
	key, err := fileKey(classes, mkey)
	if err != nil {
		return nil, nil, fmt.Errorf("the key to the backup's list of files: %w", err)
	}
	return classes, key, nil
}
