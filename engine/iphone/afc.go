package iphone

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AFC is the phone's file service: it serves the media folder,
// /var/mobile/Media, where the camera roll, the Photos library and its
// thumbnails are. Each packet has a 40-byte header, little-endian: the
// magic, the packet's whole length, the length of the header and its
// arguments, a number, and the operation; data follows the arguments.
const (
	afcMagic    = "CFA6LPAA"
	afcStatus   = 0x01
	afcReadDir  = 0x03
	afcFileInfo = 0x0a
	afcOpen     = 0x0d
	afcOpenRes  = 0x0e
	afcRead     = 0x0f
	afcSeek     = 0x11
	afcClose    = 0x14

	afcReadOnly   = 1
	afcMaxRead    = 1 << 20
	afcNotFound   = 8
	afcPermDenied = 10
)

// afc is a connection to the file service. Requests take turns.
type afc struct {
	mu  sync.Mutex
	rw  io.ReadWriteCloser
	num uint64
}

// afcError is the service's status for a request that failed.
type afcError uint64

func (e afcError) Error() string {
	switch e {
	case afcNotFound:
		return "no such file on the iPhone"
	case afcPermDenied:
		return "the iPhone doesn't allow reading that"
	}
	return fmt.Sprintf("the iPhone's file service failed (%d)", uint64(e))
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

// list lists a folder's names.
func (a *afc) list(dir string) ([]string, error) {
	_, _, data, err := a.request(afcReadDir, cstr(dir), nil)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range strings.Split(string(data), "\x00") {
		if n != "" && n != "." && n != ".." {
			out = append(out, n)
		}
	}
	return out, nil
}

// afcInfo is what the service tells of a file.
type afcInfo struct {
	dir  bool
	link bool
	size int64
	mod  time.Time
}

func (a *afc) stat(p string) (afcInfo, error) {
	_, _, data, err := a.request(afcFileInfo, cstr(p), nil)
	if err != nil {
		return afcInfo{}, err
	}
	return parseInfo(data), nil
}

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

// afcFile is a file open for reading at any offset.
type afcFile struct {
	a      *afc
	handle uint64
}

func (a *afc) open(p string) (*afcFile, error) {
	args := binary.LittleEndian.AppendUint64(nil, afcReadOnly)
	op, rargs, _, err := a.request(afcOpen, append(args, cstr(p)...), nil)
	if err != nil {
		return nil, err
	}
	if op != afcOpenRes || len(rargs) < 8 {
		return nil, fmt.Errorf("the iPhone didn't open %s", p)
	}
	return &afcFile{a: a, handle: binary.LittleEndian.Uint64(rargs)}, nil
}

// ReadAt seeks and reads. The two requests go together, so another
// read can't come between them.
func (f *afcFile) ReadAt(p []byte, off int64) (int, error) {
	n := 0
	for n < len(p) {
		got, err := f.read(off+int64(n), min(len(p)-n, afcMaxRead))
		n += copy(p[n:], got)
		if err != nil {
			return n, err
		}
		if len(got) == 0 {
			return n, io.EOF
		}
	}
	return n, nil
}

func (f *afcFile) read(off int64, want int) ([]byte, error) {
	f.a.mu.Lock()
	defer f.a.mu.Unlock()
	seek := binary.LittleEndian.AppendUint64(nil, f.handle)
	seek = binary.LittleEndian.AppendUint64(seek, 0) // from the start
	seek = binary.LittleEndian.AppendUint64(seek, uint64(off))
	if _, _, _, err := f.a.roundTrip(afcSeek, seek, nil); err != nil {
		return nil, err
	}
	args := binary.LittleEndian.AppendUint64(nil, f.handle)
	args = binary.LittleEndian.AppendUint64(args, uint64(want))
	_, _, data, err := f.a.roundTrip(afcRead, args, nil)
	return data, err
}

func (f *afcFile) Close() error {
	_, _, _, err := f.a.request(afcClose, binary.LittleEndian.AppendUint64(nil, f.handle), nil)
	return err
}
