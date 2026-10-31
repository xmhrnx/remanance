package disk

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
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

func TestFileSetEncrypted(t *testing.T) {
	plain := make([]byte, 1000)
	for i := range plain {
		plain[i] = byte(i * 7)
	}
	key := bytes.Repeat([]byte{0x42}, 32)
	b, _ := aes.NewCipher(key)
	padded := append(bytes.Clone(plain), bytes.Repeat([]byte{8}, 8)...) // PKCS#7 to 1008
	enc := make([]byte, len(padded))
	cipher.NewCBCEncrypter(b, make([]byte, 16)).CryptBlocks(enc, padded)
	path := filepath.Join(t.TempDir(), "enc")
	os.WriteFile(path, enc, 0o644)

	s := NewFileSet(Info{})
	defer s.Close()
	s.Add(path, 100) // read as it is
	off, err := s.AddEncrypted(path, int64(len(plain)), key)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range [][2]int{{0, 1000}, {0, 1}, {15, 2}, {16, 16}, {17, 300}, {990, 10}, {999, 1}, {500, 0}} {
		got := make([]byte, r[1])
		if _, err := s.ReadAt(got, off+int64(r[0])); err != nil && err != io.EOF {
			t.Fatal(err)
		}
		if !bytes.Equal(got, plain[r[0]:r[0]+r[1]]) {
			t.Errorf("at %d, %d bytes: wrong", r[0], r[1])
		}
	}
	if s.Size() != off+1000 {
		t.Errorf("size %d", s.Size())
	}
}
