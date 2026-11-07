package iphone

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"
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

// afcFile is a file open for reading at any offset.
type afcFile struct {
	a      *afc
	handle uint64
}

func (e afcError) Error() string {
	switch e {
	case afcNotFound:
		return "no such file on the iPhone"
	case afcPermDenied:
		return "the iPhone doesn't allow reading that"
	}
	return fmt.Sprintf("the iPhone's file service failed (%d)", uint64(e))
}

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

func (a *afc) stat(p string) (afcInfo, error) {
	_, _, data, err := a.request(afcFileInfo, cstr(p), nil)
	if err != nil {
		return afcInfo{}, err
	}
	return parseInfo(data), nil
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
