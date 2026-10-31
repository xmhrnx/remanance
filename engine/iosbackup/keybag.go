package iosbackup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// An encrypted backup keeps a key for each data protection class in its
// keybag, wrapped with a key made from the backup's password. Each file
// has its own key, wrapped with its class's key; Manifest.db has one
// too, in Manifest.plist. Files are AES-256 in CBC mode, from a zero IV.

// keybag is the BackupKeyBag of Manifest.plist.
type keybag struct {
	salt  []byte // SALT and ITER: PBKDF2 with SHA-1, the second round
	iter  int
	dpsl  []byte // DPSL and DPIC: PBKDF2 with SHA-256, the first round (iOS 10.2 and later)
	dpic  int
	class map[uint32]classKey
}

type classKey struct {
	wrap    uint32 // 2: wrapped with the password's key
	wrapped []byte
}

// parseKeybag reads a keybag: four-byte tags, each with a four-byte
// length and its value. The first UUID is the bag's; each later one
// starts a class key.
func parseKeybag(b []byte) (*keybag, error) {
	k := &keybag{class: map[uint32]classKey{}}
	var cur *classKey
	var curClass uint32
	uuids := 0
	end := func() {
		if cur != nil && curClass != 0 {
			k.class[curClass] = *cur
		}
	}
	for len(b) >= 8 {
		tag := string(b[:4])
		n := binary.BigEndian.Uint32(b[4:])
		if uint64(n) > uint64(len(b)-8) {
			return nil, fmt.Errorf("the backup's keybag is damaged")
		}
		v := b[8 : 8+n]
		b = b[8+n:]
		num := uint32(0)
		if len(v) == 4 {
			num = binary.BigEndian.Uint32(v)
		}
		switch tag {
		case "UUID":
			uuids++
			if uuids > 1 {
				end()
				cur, curClass = &classKey{}, 0
			}
		case "CLAS":
			curClass = num
		case "WRAP":
			if cur != nil {
				cur.wrap = num
			}
		case "WPKY":
			if cur != nil {
				cur.wrapped = v
			}
		case "SALT":
			k.salt = v
		case "ITER":
			k.iter = int(num)
		case "DPSL":
			k.dpsl = v
		case "DPIC":
			k.dpic = int(num)
		}
	}
	end()
	if len(k.salt) == 0 || k.iter == 0 || len(k.class) == 0 {
		return nil, fmt.Errorf("the backup's keybag is damaged")
	}
	return k, nil
}

// errWrong means the password doesn't unwrap the keybag's keys.
var errWrong = errors.New("wrong password")

// unlock makes the key from the password and unwraps the class keys
// with it. Making the key takes a few seconds, as it's meant to.
func (k *keybag) unlock(password string) (map[uint32][]byte, error) {
	pw := []byte(password)
	if len(k.dpsl) > 0 && k.dpic > 0 {
		var err error
		if pw, err = pbkdf2.Key(sha256.New, password, k.dpsl, k.dpic, 32); err != nil {
			return nil, err
		}
	}
	key, err := pbkdf2.Key(sha1.New, string(pw), k.salt, k.iter, 32)
	if err != nil {
		return nil, err
	}
	out := map[uint32][]byte{}
	for c, ck := range k.class {
		if ck.wrap&2 == 0 || len(ck.wrapped) != 40 {
			continue // kept on the device, not in the backup
		}
		u, ok := unwrap(key, ck.wrapped)
		if !ok {
			return nil, errWrong
		}
		out[c] = u
	}
	if len(out) == 0 {
		return nil, errWrong
	}
	return out, nil
}

// fileKey unwraps a key stored with its class as a four-byte
// little-endian prefix, as Manifest.plist's ManifestKey and each file's
// EncryptionKey are.
func fileKey(classes map[uint32][]byte, stored []byte) ([]byte, error) {
	if len(stored) < 4+24 {
		return nil, fmt.Errorf("a key of %d bytes", len(stored))
	}
	ck, ok := classes[binary.LittleEndian.Uint32(stored)]
	if !ok {
		return nil, fmt.Errorf("no key for protection class %d", binary.LittleEndian.Uint32(stored))
	}
	k, ok := unwrap(ck, stored[4:])
	if !ok {
		return nil, fmt.Errorf("a key that doesn't unwrap")
	}
	return k, nil
}

// unwrap is AES key unwrap (RFC 3394).
func unwrap(kek, wrapped []byte) ([]byte, bool) {
	b, err := aes.NewCipher(kek)
	if err != nil || len(wrapped)%8 != 0 || len(wrapped) < 24 {
		return nil, false
	}
	n := len(wrapped)/8 - 1
	a := bytes.Clone(wrapped[:8])
	r := make([][]byte, n)
	for i := range r {
		r[i] = bytes.Clone(wrapped[8*(i+1) : 8*(i+2)])
	}
	buf := make([]byte, 16)
	for j := 5; j >= 0; j-- {
		for i := n; i >= 1; i-- {
			t := uint64(n*j + i)
			binary.BigEndian.PutUint64(buf, binary.BigEndian.Uint64(a)^t)
			copy(buf[8:], r[i-1])
			b.Decrypt(buf, buf)
			copy(a, buf[:8])
			copy(r[i-1], buf[8:])
		}
	}
	if !bytes.Equal(a, bytes.Repeat([]byte{0xa6}, 8)) {
		return nil, false
	}
	return bytes.Join(r, nil), true
}

// decrypt decrypts a whole file, such as Manifest.db, and drops its
// padding.
func decrypt(key, data []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("an encrypted file of %d bytes", len(data))
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(b, make([]byte, aes.BlockSize)).CryptBlocks(out, data)
	if n := len(out); n > 0 {
		if p := int(out[n-1]); p >= 1 && p <= aes.BlockSize && p <= n {
			out = out[:n-p]
		}
	}
	return out, nil
}
