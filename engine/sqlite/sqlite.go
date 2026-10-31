// Package sqlite reads SQLite databases without SQLite: the table
// b-trees and records of the file format, read-only. iPhone backups list
// their files in one, and apps keep their data in them.
package sqlite

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrFormat means the data isn't an SQLite database this package reads.
var ErrFormat = errors.New("sqlite: not an SQLite database")

// Text encodings, as the header names them.
const (
	encUTF8    = 1
	encUTF16LE = 2
	encUTF16BE = 3
)

// DB is an open database.
type DB struct {
	r        io.ReaderAt
	size     int64
	pageSize int
	usable   int // page size less the bytes reserved at each page's end
	pages    uint32
	enc      int
	tables   map[string]*Table
	wal      map[uint32][]byte // pages the write-ahead log replaces
	frames   []walFrame        // every page image in the log
}

// Open reads the header of the database in r, size bytes long.
func Open(r io.ReaderAt, size int64) (*DB, error) {
	h := make([]byte, 100)
	if _, err := r.ReadAt(h, 0); err != nil {
		return nil, ErrFormat
	}
	if string(h[:16]) != "SQLite format 3\x00" {
		return nil, ErrFormat
	}
	ps := int(binary.BigEndian.Uint16(h[16:]))
	if ps == 1 {
		ps = 65536
	}
	if ps < 512 || ps&(ps-1) != 0 {
		return nil, ErrFormat
	}
	db := &DB{r: r, size: size, pageSize: ps, usable: ps - int(h[20]), enc: int(binary.BigEndian.Uint32(h[56:]))}
	if db.enc == 0 {
		db.enc = encUTF8
	}
	db.pages = uint32(size / int64(ps))
	if db.usable < 480 {
		return nil, ErrFormat
	}
	return db, nil
}

// page reads page n, counting from 1.
func (db *DB) page(n uint32) ([]byte, error) {
	if n == 0 || n > db.pages {
		return nil, fmt.Errorf("sqlite: page %d is outside the file", n)
	}
	if p, ok := db.wal[n]; ok {
		return p, nil
	}
	p := make([]byte, db.pageSize)
	if _, err := db.r.ReadAt(p, int64(n-1)*int64(db.pageSize)); err != nil && err != io.EOF {
		return nil, err
	}
	return p, nil
}
