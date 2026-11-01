package space

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestMeasure(t *testing.T) {
	root := t.TempDir()
	for p, n := range map[string]int{"a/big.mp4": 300000, "a/b/small.txt": 1000, "c.jpg": 50000, "empty/.keep": 0} {
		f := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(f), 0o755)
		if err := os.WriteFile(f, make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	top, err := Measure(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if top.Files != 4 || len(top.Children) != 3 {
		t.Fatalf("top: %d files, %d children", top.Files, len(top.Children))
	}
	a := top.Children[0]
	if a.Name != "a" || a.Files != 2 || a.Size < 301000 {
		t.Fatalf("largest is %s, %d bytes, %d files", a.Name, a.Size, a.Files)
	}
	if top.Size != a.Size+top.Children[1].Size+top.Children[2].Size {
		t.Fatalf("sizes don't add up")
	}
	if k := top.Kinds(); k[0].Kind != KindVideo {
		t.Fatalf("kinds: %+v", k)
	}
	big := a.Children[0]
	big.Remove()
	if top.Files != 3 || len(a.Children) != 1 {
		t.Fatalf("after removing: %d files", top.Files)
	}
}

func TestLayout(t *testing.T) {
	var nodes []*Node
	for _, s := range []int64{6, 6, 4, 3, 2, 2, 1, 0} {
		nodes = append(nodes, &Node{Size: s})
	}
	area := Rect{W: 6, H: 4}
	tiles := Layout(nodes, area)
	if len(tiles) != 7 {
		t.Fatalf("%d tiles", len(tiles))
	}
	var sum float64
	for _, tl := range tiles {
		sum += tl.W * tl.H
		want := float64(tl.Node.Size)
		if math.Abs(tl.W*tl.H-want) > 1e-9 {
			t.Errorf("tile of %v has area %v", want, tl.W*tl.H)
		}
		if tl.X < -1e-9 || tl.Y < -1e-9 || tl.X+tl.W > 6+1e-9 || tl.Y+tl.H > 4+1e-9 {
			t.Errorf("tile %+v outside the area", tl.Rect)
		}
	}
	if math.Abs(sum-24) > 1e-9 {
		t.Fatalf("tiles cover %v of 24", sum)
	}
}
