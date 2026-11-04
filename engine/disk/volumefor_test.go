package disk

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestVolumeFor(t *testing.T) {
	root, data := "/", "/mnt/data"
	if runtime.GOOS == "windows" {
		root, data = "C:", `D:\`
	}
	infos := []Info{
		{ID: "disk", Kind: KindPhysical, Mounts: []string{root}},
		{ID: "root", Kind: KindVolume, Mounts: []string{root}},
		{ID: "data", Kind: KindVolume, Mounts: []string{data}},
	}
	top := root
	if runtime.GOOS == "windows" {
		top = `C:\`
	}
	for _, c := range []struct {
		dir, id, rel string
	}{
		{filepath.Join(top, "Users", "me", "Documents"), "root", "Users/me/Documents"},
		{filepath.Join(data, "Music"), "data", "Music"},
		{data, "data", ""},
	} {
		in, rel, ok := VolumeFor(infos, c.dir)
		if !ok || in.ID != c.id || rel != c.rel {
			t.Errorf("%s: got %s %q %v, want %s %q", c.dir, in.ID, rel, ok, c.id, c.rel)
		}
	}
	if runtime.GOOS == "windows" {
		if _, _, ok := VolumeFor(infos, `E:\Photos`); ok {
			t.Error("E: has no volume, but one was found")
		}
		if in, _, _ := VolumeFor(infos, `c:\users`); in.ID != "root" {
			t.Error("drive letters should match whatever their case")
		}
	}
}
