package iosbackup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

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
