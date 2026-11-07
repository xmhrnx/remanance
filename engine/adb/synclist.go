package adb

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"time"
)

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
