package rename

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func files(t *testing.T, names ...string) (string, []string) {
	dir := t.TempDir()
	var ps []string
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
		ps = append(ps, p)
	}
	return dir, ps
}

func names(cs []Change) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.New)
	}
	return out
}

func TestPlan(t *testing.T) {
	_, ps := files(t, "IMG_0001.JPG", "IMG_0002.JPG", "holiday notes.TXT")
	cs, err := Plan(ps, Rules{Pattern: "Trip {n:3} - {name}", Start: 1, Find: "img_", Replace: "", Case: CaseTitle, LowerExt: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Trip 001 - 0001.jpg", "Trip 002 - 0002.jpg", "Trip 003 - Holiday Notes.txt"}
	for i, n := range names(cs) {
		if n != want[i] || cs[i].Problem != "" {
			t.Errorf("%d: %q (%s), want %q", i, n, cs[i].Problem, want[i])
		}
	}

	cs, _ = Plan(ps, Rules{Pattern: "same"})
	if cs[0].Problem == "" || cs[1].Problem == "" {
		t.Errorf("clash not found: %+v", cs)
	}
	cs, _ = Plan(ps[:1], Rules{Pattern: "a:b"})
	if cs[0].Problem == "" {
		t.Errorf("colon allowed")
	}
	cs, _ = Plan(ps[:1], Rules{Pattern: "holiday notes", Ext: "TXT"})
	if cs[0].Problem == "" {
		t.Errorf("existing file not found")
	}
	cs, _ = Plan(ps[:1], Rules{Find: `IMG_(\d+)`, Replace: "photo-$1", Regexp: true, MatchCase: true})
	if cs[0].New != "photo-0001.JPG" {
		t.Errorf("regexp: %q", cs[0].New)
	}
	if _, err := Plan(ps, Rules{Find: "(", Regexp: true}); err == nil {
		t.Errorf("bad regexp accepted")
	}
}

func TestApplySwapAndUndo(t *testing.T) {
	dir, ps := files(t, "a.txt", "b.txt")
	cs := []Change{{Path: ps[0], New: "b.txt"}, {Path: ps[1], New: "a.txt"}}
	check(cs)
	done, err := Apply(cs)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "b.txt")); string(b) != "a.txt" {
		t.Fatalf("b.txt holds %q", b)
	}
	if err := Undo(done); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "a.txt" {
		t.Fatalf("after undo a.txt holds %q", b)
	}
	if left, _ := os.ReadDir(dir); len(left) != 2 {
		t.Fatalf("%d files left", len(left))
	}
}

func TestPhotoDate(t *testing.T) {
	// A JPEG with an EXIF block whose EXIF directory has DateTimeOriginal.
	bo := binary.LittleEndian
	tiff := []byte("II*\x00")
	tiff = bo.AppendUint32(tiff, 8)
	tiff = bo.AppendUint16(tiff, 1) // IFD0: one entry, the EXIF pointer
	tiff = bo.AppendUint16(tiff, 0x8769)
	tiff = bo.AppendUint16(tiff, 4)
	tiff = bo.AppendUint32(tiff, 1)
	tiff = bo.AppendUint32(tiff, 26)
	tiff = bo.AppendUint32(tiff, 0)
	tiff = bo.AppendUint16(tiff, 1) // the EXIF directory, at 26
	tiff = bo.AppendUint16(tiff, 0x9003)
	tiff = bo.AppendUint16(tiff, 2)
	tiff = bo.AppendUint32(tiff, 20)
	tiff = bo.AppendUint32(tiff, 44)
	tiff = bo.AppendUint32(tiff, 0)
	tiff = append(tiff, "2019:07:14 18:30:05\x00"...)
	seg := append([]byte("Exif\x00\x00"), tiff...)
	jpg := []byte{0xFF, 0xD8, 0xFF, 0xE1}
	jpg = binary.BigEndian.AppendUint16(jpg, uint16(len(seg)+2))
	jpg = append(append(jpg, seg...), 0xFF, 0xD9)
	p := filepath.Join(t.TempDir(), "x.jpg")
	os.WriteFile(p, jpg, 0o644)
	got, ok := photoDate(p)
	want := time.Date(2019, 7, 14, 18, 30, 5, 0, time.Local)
	if !ok || !got.Equal(want) {
		t.Fatalf("photoDate = %v, %v", got, ok)
	}
	cs, _ := Plan([]string{p}, Rules{Pattern: "{date:2006-01-02 15.04}"})
	if cs[0].New != "2019-07-14 18.30.jpg" {
		t.Fatalf("named %q", cs[0].New)
	}
}
