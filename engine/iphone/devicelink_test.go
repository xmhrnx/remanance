package iphone

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/xmhrnx/remanance/engine/plist"
)

// phone is the phone's end of a pipe, for tests.
type phone struct {
	t *testing.T
	c *conn
}

func pipe(t *testing.T) (*link, *phone) {
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	return &link{c: &conn{a}, root: t.TempDir()}, &phone{t: t, c: &conn{b}}
}

func (p *phone) u32(n uint32) { p.c.Write(binary.BigEndian.AppendUint32(nil, n)) }

func (p *phone) readU32() uint32 {
	b := make([]byte, 4)
	if _, err := io.ReadFull(p.c, b); err != nil {
		p.t.Error(err)
	}
	return binary.BigEndian.Uint32(b)
}

func (p *phone) status() []any {
	v, err := p.c.recv()
	if err != nil {
		p.t.Fatal(err)
	}
	m, _ := v.([]any)
	if len(m) != 4 || m[0] != "DLMessageStatusResponse" {
		p.t.Fatalf("status %v", v)
	}
	return m
}

func TestReceiveFiles(t *testing.T) {
	l, ph := pipe(t)
	done := make(chan error)
	var sizes []int64
	go func() { done <- l.receiveFiles(func(n int64) { sizes = append(sizes, n) }) }()
	send := func(name string, blocks ...[]byte) {
		for _, n := range []string{"device/" + name, name} {
			ph.u32(uint32(len(n)))
			ph.c.Write([]byte(n))
		}
		for _, b := range blocks {
			ph.u32(uint32(len(b) + 1))
			ph.c.Write(append([]byte{codeFileData}, b...))
		}
		ph.u32(1)
		ph.c.Write([]byte{codeSuccess})
	}
	send("UDID/ab/abcdef", []byte("first "), []byte("second"))
	send("UDID/Manifest.db", bytes.Repeat([]byte{7}, 70000))
	ph.u32(0)
	if m := ph.status(); m[1] != int64(0) {
		t.Fatalf("status %v", m)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(l.root, "UDID", "ab", "abcdef")); string(got) != "first second" {
		t.Errorf("abcdef: %q", got)
	}
	if st, err := os.Stat(filepath.Join(l.root, "UDID", "Manifest.db")); err != nil || st.Size() != 70000 {
		t.Errorf("Manifest.db: %v %v", st, err)
	}
	if len(sizes) != 2 || sizes[1] != 70000 {
		t.Errorf("sizes %v", sizes)
	}
}

func TestSendFiles(t *testing.T) {
	l, ph := pipe(t)
	os.MkdirAll(filepath.Join(l.root, "UDID"), 0o755)
	os.WriteFile(filepath.Join(l.root, "UDID", "Status.plist"), []byte("status"), 0o644)
	done := make(chan error)
	go func() { done <- l.sendFiles([]any{"UDID/Status.plist", "UDID/Manifest.plist"}) }()
	read := func() (string, []byte, byte) {
		n := ph.readU32()
		name := make([]byte, n)
		io.ReadFull(ph.c, name)
		var data []byte
		for {
			n := ph.readU32()
			b := make([]byte, n)
			io.ReadFull(ph.c, b)
			if b[0] != codeFileData {
				return string(name), data, b[0]
			}
			data = append(data, b[1:]...)
		}
	}
	if name, data, code := read(); name != "UDID/Status.plist" || string(data) != "status" || code != codeSuccess {
		t.Errorf("%s: %q %d", name, data, code)
	}
	if name, _, code := read(); name != "UDID/Manifest.plist" || code != codeError {
		t.Errorf("%s: %d", name, code)
	}
	if n := ph.readU32(); n != 0 {
		t.Errorf("ended with %d", n)
	}
	m := ph.status()
	if m[1] != int64(-13) || plist.Dict(m[3])["UDID/Manifest.plist"] == nil {
		t.Errorf("status %v", m)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMoveRemove(t *testing.T) {
	l, ph := pipe(t)
	os.MkdirAll(filepath.Join(l.root, "UDID", "Snapshot"), 0o755)
	os.WriteFile(filepath.Join(l.root, "UDID", "Snapshot", "Manifest.db"), []byte("db"), 0o644)
	go func() {
		l.move(map[string]any{"UDID/Snapshot/Manifest.db": "UDID/Manifest.db"})
		l.remove([]any{"UDID/Snapshot"})
		l.move(map[string]any{"../outside": "UDID/x"})
	}()
	ph.status()
	ph.status()
	if got, _ := os.ReadFile(filepath.Join(l.root, "UDID", "Manifest.db")); string(got) != "db" {
		t.Errorf("moved: %q", got)
	}
	if _, err := os.Stat(filepath.Join(l.root, "UDID", "Snapshot")); err == nil {
		t.Error("not removed")
	}
}

func TestHandshake(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	ph := &conn{b}
	done := make(chan error)
	go func() { done <- handshake(&conn{a}) }()
	ph.sendBinary([]any{"DLMessageVersionExchange", int64(300), int64(0)})
	if v, _ := ph.recv(); v.([]any)[1] != "DLVersionsOk" {
		t.Fatalf("%v", v)
	}
	ph.sendBinary([]any{"DLMessageDeviceReady"})
	v, _ := ph.recv()
	if plist.Dict(v.([]any)[1])["MessageName"] != "Hello" {
		t.Fatalf("%v", v)
	}
	ph.sendBinary([]any{"DLMessageProcessMessage", map[string]any{"ErrorCode": int64(0), "ProtocolVersion": 2.1}})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMux(t *testing.T) {
	var b bytes.Buffer
	writeMux(&b, 7, []byte("<plist/>"))
	if h := b.Bytes(); binary.LittleEndian.Uint32(h) != 24 || binary.LittleEndian.Uint32(h[8:]) != 8 || binary.LittleEndian.Uint32(h[12:]) != 7 {
		t.Fatalf("header % x", h[:16])
	}
	body, err := readMux(&b)
	if err != nil || string(body) != "<plist/>" {
		t.Fatalf("%q %v", body, err)
	}
}
