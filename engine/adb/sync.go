package adb

import (
	"errors"
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

func (e entry) Name() string { return e.name }

func (e entry) IsDir() bool { return e.mode&0o170000 == 0o040000 }

func (e entry) Type() fs.FileMode { return e.fileMode().Type() }

func (e entry) Info() (fs.FileInfo, error) { return e, nil }

func (e entry) Size() int64 { return e.size }

func (e entry) ModTime() time.Time { return e.mod }

func (e entry) Sys() any { return nil }

func (e entry) Mode() fs.FileMode { return e.fileMode() }

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
