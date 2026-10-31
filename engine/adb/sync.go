package adb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"path"
	"strings"
	"sync"
	"time"
)

// storageFS lists a folder of the phone through adb's file sync
// service, as an fs.FS whose paths are relative to it.
type storageFS struct {
	serial string
	root   string // "/storage/emulated/0"
	v2     bool   // the phone lists with LIS2, with 64-bit sizes

	mu   sync.Mutex
	conn net.Conn
}

func newStorageFS(serial, root string) *storageFS {
	return &storageFS{serial: serial, root: root, v2: strings.Contains(features(serial), "ls_v2")}
}

// Open isn't how this FS is read: fs.ReadDir uses ReadDir.
func (s *storageFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: errors.ErrUnsupported}
}

// ReadDir lists a folder, one request on the sync connection.
func (s *storageFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	full := s.root
	if name != "." {
		full = path.Join(s.root, name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		c, err := open(s.serial, "sync:")
		if err != nil {
			return nil, err
		}
		s.conn = c
	}
	s.conn.SetDeadline(time.Now().Add(time.Minute))
	out, err := list(s.conn, full, s.v2)
	if err != nil {
		// A broken connection opens again next time.
		s.conn.Close()
		s.conn = nil
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	return out, nil
}

// Close ends the sync connection.
func (s *storageFS) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
}

// list asks for a folder's entries: "LIST" or "LIS2", the path's length
// and the path; each entry comes back as "DENT" or "DNT2" with its mode,
// size and date, then its name; "DONE" ends them.
func list(c io.ReadWriter, dir string, v2 bool) ([]fs.DirEntry, error) {
	req := "LIST"
	if v2 {
		req = "LIS2"
	}
	msg := append([]byte(req), binary.LittleEndian.AppendUint32(nil, uint32(len(dir)))...)
	if _, err := c.Write(append(msg, dir...)); err != nil {
		return nil, err
	}
	var out []fs.DirEntry
	for {
		id := make([]byte, 4)
		if _, err := io.ReadFull(c, id); err != nil {
			return nil, err
		}
		var e entry
		switch string(id) {
		case "DONE":
			// With an entry's worth of zeros.
			n := 16
			if v2 {
				n = 72
			}
			_, err := io.ReadFull(c, make([]byte, n))
			return out, err
		case "DENT":
			b := make([]byte, 16)
			if _, err := io.ReadFull(c, b); err != nil {
				return nil, err
			}
			e.mode = binary.LittleEndian.Uint32(b)
			e.size = int64(binary.LittleEndian.Uint32(b[4:]))
			e.mod = time.Unix(int64(binary.LittleEndian.Uint32(b[8:])), 0)
			if err := readName(c, &e, binary.LittleEndian.Uint32(b[12:])); err != nil {
				return nil, err
			}
		case "DNT2":
			b := make([]byte, 72)
			if _, err := io.ReadFull(c, b); err != nil {
				return nil, err
			}
			// error, dev, ino, mode, nlink, uid, gid, size, atime,
			// mtime, ctime, name length.
			e.mode = binary.LittleEndian.Uint32(b[20:])
			e.size = int64(binary.LittleEndian.Uint64(b[36:]))
			e.mod = time.Unix(int64(binary.LittleEndian.Uint64(b[52:])), 0)
			if err := readName(c, &e, binary.LittleEndian.Uint32(b[68:])); err != nil {
				return nil, err
			}
		case "FAIL":
			b := make([]byte, 4)
			if _, err := io.ReadFull(c, b); err != nil {
				return nil, err
			}
			msg := make([]byte, min(binary.LittleEndian.Uint32(b), 4096))
			io.ReadFull(c, msg)
			return nil, fmt.Errorf("%s", msg)
		default:
			return nil, fmt.Errorf("the phone's file service sent %q", id)
		}
		if e.name != "." && e.name != ".." {
			out = append(out, e)
		}
	}
}

func readName(c io.Reader, e *entry, n uint32) error {
	if n > 4096 {
		return fmt.Errorf("a file name of %d bytes", n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(c, b); err != nil {
		return err
	}
	e.name = string(b)
	return nil
}

// entry is a file or folder as the sync service lists it.
type entry struct {
	name string
	mode uint32 // Unix
	size int64
	mod  time.Time
}

func (e entry) Name() string               { return e.name }
func (e entry) IsDir() bool                { return e.mode&0o170000 == 0o040000 }
func (e entry) Type() fs.FileMode          { return e.fileMode().Type() }
func (e entry) Info() (fs.FileInfo, error) { return e, nil }
func (e entry) Size() int64                { return e.size }
func (e entry) ModTime() time.Time         { return e.mod }
func (e entry) Sys() any                   { return nil }
func (e entry) Mode() fs.FileMode          { return e.fileMode() }

func (e entry) fileMode() fs.FileMode {
	m := fs.FileMode(e.mode & 0o777)
	switch e.mode & 0o170000 {
	case 0o040000:
		m |= fs.ModeDir
	case 0o120000:
		m |= fs.ModeSymlink
	case 0o100000:
	default:
		m |= fs.ModeIrregular
	}
	return m
}
