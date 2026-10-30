package disk

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSet(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := bytes.Repeat([]byte("a"), 5000)
	b := []byte("bbb")
	s := NewFileSet(Info{ID: "files:test", Kind: KindFiles})
	offA := s.Add(write("a", a), int64(len(a)))
	defer s.Close()
	offB := s.Add(write("b", b), int64(len(b)))
	if offA != 0 || offB != 8192 || s.Size() != 8195 {
		t.Fatalf("offsets %d %d, size %d", offA, offB, s.Size())
	}
	// A read across the end of a, the gap and into b.
	p := make([]byte, 4000)
	n, err := s.ReadAt(p, 4195)
	if n != 4000 || err != nil {
		t.Fatalf("read %d, %v", n, err)
	}
	if !bytes.Equal(p[:805], a[4195:]) || p[805] != 0 || p[3996] != 0 || !bytes.Equal(p[3997:], b) {
		t.Fatalf("read the wrong bytes")
	}
	// More files than stay open at once.
	for i := range maxOpen + 4 {
		s.Add(write("f"+string(rune('a'+i)), []byte{byte(i)}), 1)
	}
	for i := range maxOpen + 4 {
		q := make([]byte, 1)
		if _, err := s.ReadAt(q, s.files[2+i].off); err != nil || q[0] != byte(i) {
			t.Fatalf("file %d: %v %v", i, q, err)
		}
	}
	if len(s.open) > maxOpen {
		t.Fatalf("%d files open", len(s.open))
	}
}

func TestFileSetEOF(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	os.WriteFile(p, []byte("xyz"), 0o644)
	s := NewFileSet(Info{})
	defer s.Close()
	s.Add(p, 3)
	buf := make([]byte, 10)
	if n, err := s.ReadAt(buf, 1); n != 2 || err != io.EOF {
		t.Fatalf("read %d, %v", n, err)
	}
}
