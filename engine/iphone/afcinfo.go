package iphone

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"time"
)

// afcInfo is what the service tells of a file.
type afcInfo struct {
	dir  bool
	link bool
	size int64
	mod  time.Time
}

// request sends an operation and reads the reply: its arguments and its
// data.
func (a *afc) request(op uint64, args, data []byte) (uint64, []byte, []byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.roundTrip(op, args, data)
}

// roundTrip is request, with the lock held.
func (a *afc) roundTrip(op uint64, args, data []byte) (uint64, []byte, []byte, error) {
	a.num++
	h := make([]byte, 40, 40+len(args)+len(data))
	copy(h, afcMagic)
	binary.LittleEndian.PutUint64(h[8:], uint64(40+len(args)+len(data)))
	binary.LittleEndian.PutUint64(h[16:], uint64(40+len(args)))
	binary.LittleEndian.PutUint64(h[24:], a.num)
	binary.LittleEndian.PutUint64(h[32:], op)
	if _, err := a.rw.Write(append(append(h, args...), data...)); err != nil {
		return 0, nil, nil, err
	}
	if _, err := io.ReadFull(a.rw, h[:40]); err != nil {
		return 0, nil, nil, err
	}
	if string(h[:8]) != afcMagic {
		return 0, nil, nil, fmt.Errorf("the iPhone's file service sent a bad packet")
	}
	whole, head := binary.LittleEndian.Uint64(h[8:]), binary.LittleEndian.Uint64(h[16:])
	if head < 40 || whole < head || whole > 64<<20 {
		return 0, nil, nil, fmt.Errorf("the iPhone's file service sent a packet of %d bytes", whole)
	}
	body := make([]byte, whole-40)
	if _, err := io.ReadFull(a.rw, body); err != nil {
		return 0, nil, nil, err
	}
	rop := binary.LittleEndian.Uint64(h[32:])
	rargs, rdata := body[:head-40], body[head-40:]
	if rop == afcStatus && len(rargs) >= 8 {
		if code := binary.LittleEndian.Uint64(rargs); code != 0 {
			return rop, rargs, rdata, afcError(code)
		}
	}
	return rop, rargs, rdata, nil
}

func cstr(s string) []byte { return append([]byte(s), 0) }

// parseInfo reads the key and value pairs of a file's info.
func parseInfo(data []byte) afcInfo {
	var i afcInfo
	f := bytes.Split(data, []byte{0})
	for k := 0; k+1 < len(f); k += 2 {
		v := string(f[k+1])
		switch string(f[k]) {
		case "st_size":
			i.size, _ = strconv.ParseInt(v, 10, 64)
		case "st_ifmt":
			i.dir, i.link = v == "S_IFDIR", v == "S_IFLNK"
		case "st_mtime":
			if ns, err := strconv.ParseInt(v, 10, 64); err == nil {
				i.mod = time.Unix(0, ns)
			}
		}
	}
	return i
}
