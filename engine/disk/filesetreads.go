package disk

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
)

func (s *FileSet) readFile(i int, p []byte, off int64) (int, error) {
	if b := s.files[i].cbc; b != nil {
		return s.readDecrypted(i, b, p, off)
	}
	return s.readPlain(i, p, off)
}

// readDecrypted reads the whole blocks under p, and the one before them,
// whose ciphertext is the next one's IV.
func (s *FileSet) readDecrypted(i int, b cipher.Block, p []byte, off int64) (int, error) {
	const bs = aes.BlockSize
	start := off &^ (bs - 1)
	end := (off + int64(len(p)) + bs - 1) &^ (bs - 1)
	from := max(start-bs, 0)
	enc := make([]byte, end-from)
	n, err := s.readPlain(i, enc, from)
	n -= n % bs
	iv := make([]byte, bs)
	if start > 0 {
		if n < bs {
			return 0, err
		}
		copy(iv, enc[:bs])
		enc, n = enc[bs:], n-bs
	}
	if n <= 0 {
		return 0, err
	}
	cipher.NewCBCDecrypter(b, iv).CryptBlocks(enc[:n], enc[:n])
	skip := int(off - start)
	if skip >= n {
		return 0, err
	}
	m := copy(p, enc[skip:n])
	if m < len(p) && err == nil {
		err = io.EOF
	}
	return m, err
}

func (s *FileSet) readPlain(i int, p []byte, off int64) (int, error) {
	s.mu.Lock()
	f, ok := s.open[i]
	if !ok {
		var err error
		if f, err = s.opener(s.files[i].path); err != nil {
			s.mu.Unlock()
			return 0, err
		}
		s.open[i] = f
		if len(s.used) >= maxOpen {
			old := s.used[0]
			s.used = s.used[1:]
			s.open[old].Close()
			delete(s.open, old)
		}
	} else {
		for j, u := range s.used {
			if u == i {
				s.used = append(s.used[:j], s.used[j+1:]...)
				break
			}
		}
	}
	s.used = append(s.used, i)
	// Reading under the lock keeps another read from closing f mid-way.
	defer s.mu.Unlock()
	return f.ReadAt(p, off)
}
