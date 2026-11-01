package dupes

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// scene draws a picture with shapes in it, w by h.
func scene(w, h int, seed int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			v := uint8(255 * fx)
			if (fx-0.3)*(fx-0.3)+(fy-0.4)*(fy-0.4) < 0.04 {
				v = 30
			}
			if seed == 1 && fy > 0.6 {
				v = 255 - uint8(255*fy)
			}
			img.Set(x, y, color.RGBA{v, uint8(255 * fy), 128, 255})
		}
	}
	return img
}

func write(t *testing.T, p string, data []byte) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func encode(t *testing.T, img image.Image, asJPEG bool) []byte {
	var b bytes.Buffer
	if asJPEG {
		jpeg.Encode(&b, img, &jpeg.Options{Quality: 60})
	} else {
		png.Encode(&b, img)
	}
	return b.Bytes()
}

// mp4 makes the boxes of a video's header: its length and frame size.
func mp4(secs, w, h uint32) []byte {
	box := func(typ string, body []byte) []byte {
		b := binary.BigEndian.AppendUint32(nil, uint32(8+len(body)))
		return append(append(b, typ...), body...)
	}
	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:], 1000)
	binary.BigEndian.PutUint32(mvhd[16:], secs*1000)
	tkhd := make([]byte, 84)
	binary.BigEndian.PutUint32(tkhd[76:], w<<16)
	binary.BigEndian.PutUint32(tkhd[80:], h<<16)
	moov := box("moov", append(box("mvhd", mvhd), box("trak", box("tkhd", tkhd))...))
	return append(box("ftyp", []byte("isom0000")), moov...)
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	big := scene(320, 240, 0)
	write(t, filepath.Join(root, "a/report.pdf"), bytes.Repeat([]byte("report"), 5000))
	write(t, filepath.Join(root, "b/report copy.pdf"), bytes.Repeat([]byte("report"), 5000))
	write(t, filepath.Join(root, "b/other.pdf"), bytes.Repeat([]byte("rePort"), 5000))
	write(t, filepath.Join(root, "photos/beach.png"), encode(t, big, false))
	small := image.NewRGBA(image.Rect(0, 0, 160, 120))
	for y := range 120 {
		for x := range 160 {
			small.Set(x, y, big.At(x*2, y*2))
		}
	}
	write(t, filepath.Join(root, "photos/beach-small.jpg"), encode(t, small, true))
	write(t, filepath.Join(root, "photos/different.png"), encode(t, scene(320, 240, 1), false))
	write(t, filepath.Join(root, "videos/clip.mp4"), mp4(12, 1920, 1080))
	write(t, filepath.Join(root, "videos/clip-720.mp4"), append(mp4(12, 1280, 720), 0))
	write(t, filepath.Join(root, "videos/other.mp4"), mp4(30, 1920, 1080))

	groups, err := Find(context.Background(), []string{root}, Options{Similar: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[Kind][]Group{}
	for _, g := range groups {
		kinds[g.Kind] = append(kinds[g.Kind], g)
	}
	if len(kinds[Exact]) != 1 || len(kinds[Exact][0].Files) != 2 {
		t.Fatalf("exact: %+v", kinds[Exact])
	}
	if len(kinds[SimilarPhoto]) != 1 || len(kinds[SimilarPhoto][0].Files) != 2 {
		t.Fatalf("photos: %+v", kinds[SimilarPhoto])
	}
	if g := kinds[SimilarPhoto][0]; filepath.Base(g.Files[0].Path) != "beach.png" || !g.Files[0].Keep || g.Files[1].Keep {
		t.Fatalf("keeps %+v", g.Files)
	}
	if len(kinds[SimilarVideo]) != 1 || len(kinds[SimilarVideo][0].Files) != 2 {
		t.Fatalf("videos: %+v", kinds[SimilarVideo])
	}
	if g := kinds[SimilarVideo][0]; g.Files[0].Width != 1920 {
		t.Fatalf("keeps the smaller video: %+v", g.Files)
	}
}
