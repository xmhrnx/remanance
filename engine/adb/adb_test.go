package adb

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

// rw answers a request with canned bytes.
type rw struct {
	io.Reader
	sent bytes.Buffer
}

func (r *rw) Write(p []byte) (int, error) { return r.sent.Write(p) }

func dent(name string, mode, size, mtime uint32) []byte {
	b := []byte("DENT")
	for _, v := range []uint32{mode, size, mtime, uint32(len(name))} {
		b = binary.LittleEndian.AppendUint32(b, v)
	}
	return append(b, name...)
}

func dnt2(name string, mode uint32, size uint64, mtime int64) []byte {
	b := []byte("DNT2")
	st := make([]byte, 72)
	binary.LittleEndian.PutUint32(st[20:], mode)
	binary.LittleEndian.PutUint64(st[36:], size)
	binary.LittleEndian.PutUint64(st[52:], uint64(mtime))
	binary.LittleEndian.PutUint32(st[68:], uint32(len(name)))
	return append(append(b, st...), name...)
}

func TestList(t *testing.T) {
	var reply []byte
	reply = append(reply, dent(".", 0o40755, 0, 0)...)
	reply = append(reply, dent("DCIM", 0o40771, 4096, 1700000000)...)
	reply = append(reply, dent(".trashed-1790000000-IMG.jpg", 0o100660, 12345, 1700000001)...)
	reply = append(reply, append([]byte("DONE"), make([]byte, 16)...)...)
	c := &rw{Reader: bytes.NewReader(reply)}
	out, err := list(c, "/storage/emulated/0", false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(c.sent.Bytes(), []byte("LIST\x13\x00\x00\x00/storage/emulated/0")) {
		t.Errorf("sent %q", c.sent.Bytes())
	}
	if len(out) != 2 || !out[0].IsDir() || out[1].IsDir() || !out[1].Type().IsRegular() {
		t.Fatalf("%v", out)
	}
	if info, _ := out[1].Info(); info.Size() != 12345 || info.ModTime().Unix() != 1700000001 {
		t.Errorf("%v", info)
	}

	reply = append(dnt2("movie.mp4", 0o100660, 5<<30, 1700000002), append([]byte("DONE"), make([]byte, 72)...)...)
	out, err = list(&rw{Reader: bytes.NewReader(reply)}, "/x", true)
	if err != nil {
		t.Fatal(err)
	}
	if info, _ := out[0].Info(); len(out) != 1 || info.Size() != 5<<30 || out[0].Name() != "movie.mp4" {
		t.Errorf("%v", out)
	}

	fail := append([]byte("FAIL"), binary.LittleEndian.AppendUint32(nil, 6)...)
	if _, err := list(&rw{Reader: bytes.NewReader(append(fail, "denied"...))}, "/x", false); err == nil || err.Error() != "denied" {
		t.Errorf("%v", err)
	}
}

func TestDevices(t *testing.T) {
	ds := parseDevices("R58M12345 device usb:1-1 product:beyond1 model:SM_G973F device:beyond1 transport_id:3\n" +
		"emulator-5554 unauthorized transport_id:4\n")
	if len(ds) != 2 || ds[0].Model != "SM G973F" || ds[0].State != "device" || ds[1].State != "unauthorized" || ds[1].Model != "Android phone" {
		t.Errorf("%+v", ds)
	}
}

func TestStatus(t *testing.T) {
	if err := status(bytes.NewReader([]byte("OKAY"))); err != nil {
		t.Error(err)
	}
	if err := status(bytes.NewReader([]byte("FAIL0010device not found"))); err == nil || err.Error() != "adb: device not found" {
		t.Error(err)
	}
}
