package disk

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
	"os"
	"sort"
	"sync"
)

// FileSet is a device made of files rather than sectors: a phone's
// shared storage, an iPhone backup. Each file added gets its own stretch
// of a virtual address space, starting on a 4 KB boundary, so the engine
// can describe a file as a run on the device and recover, preview and
// carve it like any other. The space between files reads as zeros.
type FileSet struct {
	info   Info
	files  []setFile
	size   int64
	opener Opener

	mu   sync.Mutex
	open map[int]File // a few open files, closed least recently used
	used []int
}

// File is an open file of a set.
type File interface {
	io.ReaderAt
	io.Closer
}

// Opener opens a file of a set by the path it was added with: on this
// computer, or on a phone over USB.
type Opener func(path string) (File, error)

type setFile struct {
	path      string
	off, size int64
	cbc       cipher.Block // decrypts the file, for AddEncrypted
}

// maxOpen is how many of a set's files stay open between reads.
const maxOpen = 16

// NewFileSet returns an empty set described by info; Size and SectorSize
// are filled in as files are added.
func NewFileSet(info Info) *FileSet {
	return NewFileSetWith(info, func(path string) (File, error) { return os.Open(path) })
}

// NewFileSetWith returns an empty set whose files open opens.
func NewFileSetWith(info Info, open Opener) *FileSet {
	info.SectorSize = 512
	return &FileSet{info: info, opener: open, open: map[int]File{}}
}

// Add places the file at path, size bytes long, in the set and returns
// where it starts.
func (s *FileSet) Add(path string, size int64) int64 {
	off := (s.size + 4095) &^ 4095
	s.files = append(s.files, setFile{path: path, off: off, size: size})
	s.size = off + size
	s.info.Size = s.size
	return off
}

// AddEncrypted places a file encrypted with AES in CBC mode, from a
// zero IV, as an iPhone's encrypted backup keeps them; size is how long
// it is decrypted. It reads decrypted.
func (s *FileSet) AddEncrypted(path string, size int64, key []byte) (int64, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}
	off := s.Add(path, size)
	s.files[len(s.files)-1].cbc = b
	return off, nil
}

// ReadAt reads the files the range covers.
func (s *FileSet) ReadAt(p []byte, off int64) (int, error) {
	if off >= s.size {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) && off < s.size {
		// The first file ending after off.
		i := sort.Search(len(s.files), func(i int) bool { return s.files[i].off+s.files[i].size > off })
		if i == len(s.files) {
			break
		}
		f := s.files[i]
		if off < f.off {
			// A gap before the file.
			k := int(min(f.off-off, int64(len(p)-n)))
			clear(p[n : n+k])
			n += k
			off += int64(k)
			continue
		}
		k := int(min(f.off+f.size-off, int64(len(p)-n)))
		m, err := s.readFile(i, p[n:n+k], off-f.off)
		if m < k {
			// A file that shrank or can't be read reads as zeros past
			// what it had, so one file doesn't fail a whole read.
			clear(p[n+m : n+k])
			if IsGone(err) {
				return n + m, err
			}
		}
		n += k
		off += int64(k)
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// Info describes the set.
func (s *FileSet) Info() Info { return s.info }

// Size is the end of the last file.
func (s *FileSet) Size() int64 { return s.size }

// SectorSize is 512, which every reader of devices expects.
func (s *FileSet) SectorSize() int { return 512 }

// Close closes the files left open.
func (s *FileSet) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, f := range s.open {
		f.Close()
		delete(s.open, i)
	}
	s.used = nil
	return nil
}
