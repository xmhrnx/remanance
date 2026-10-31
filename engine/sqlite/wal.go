package sqlite

import (
	"encoding/binary"
	"io"
)

// walFrame is one page image in a write-ahead log.
type walFrame struct {
	page uint32
	data []byte
}

// UseWAL reads the database's write-ahead log (the "-wal" file beside
// it), size bytes long. Pages the log has committed replace the file's,
// as SQLite would read them; every page image in the log, the replaced
// and uncommitted ones too, is searched for deleted rows.
func (db *DB) UseWAL(r io.ReaderAt, size int64) error {
	h := make([]byte, 32)
	if _, err := r.ReadAt(h, 0); err != nil {
		return ErrFormat
	}
	if m := binary.BigEndian.Uint32(h); m != 0x377f0682 && m != 0x377f0683 {
		return ErrFormat
	}
	if int(binary.BigEndian.Uint32(h[8:])) != db.pageSize {
		return ErrFormat
	}
	salt1, salt2 := binary.BigEndian.Uint32(h[16:]), binary.BigEndian.Uint32(h[20:])
	frame := int64(24 + db.pageSize)
	db.wal = map[uint32][]byte{}
	pending := map[uint32][]byte{}
	for off := int64(32); off+frame <= size; off += frame {
		f := make([]byte, frame)
		if _, err := r.ReadAt(f, off); err != nil && err != io.EOF {
			return err
		}
		n := binary.BigEndian.Uint32(f)
		if n == 0 {
			continue
		}
		data := f[24:]
		db.frames = append(db.frames, walFrame{page: n, data: data})
		// A frame from an earlier run of the log, still in the file past
		// where this one ends, is history only.
		if binary.BigEndian.Uint32(f[8:]) != salt1 || binary.BigEndian.Uint32(f[12:]) != salt2 {
			continue
		}
		pending[n] = data
		if commit := binary.BigEndian.Uint32(f[4:]); commit != 0 {
			for p, d := range pending {
				db.wal[p] = d
			}
			clear(pending)
			db.pages = commit
		}
	}
	return nil
}
