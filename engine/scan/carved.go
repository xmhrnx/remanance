package scan

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/remanance/engine/carve"
	"github.com/xmhrnx/remanance/engine/fsys"
)

// carvedVolume arranges found files as Category/Type/file, naming each by
// the date in its own metadata when it has one, else by where it was.
func carvedVolume(found []carve.Found, size int64) *fsys.Volume {
	v := fsys.NewVolume("Signatures", 0, size)
	v.Label = "Found by signature"
	folders := map[string]*fsys.Entry{}
	folder := func(parent *fsys.Entry, key, name string) *fsys.Entry {
		if f, ok := folders[key]; ok {
			return f
		}
		f := &fsys.Entry{Name: name, IsDir: true, State: fsys.Carved, Health: 100}
		parent.Add(f)
		folders[key] = f
		return f
	}
	names := map[string]int{}
	for _, f := range found {
		cat := string(carve.CategoryOf(f))
		typ := carve.TypeName(f.Ext)
		dir := folder(folder(v.Root, cat, cat), cat+"/"+typ, typ)
		base := f.Name
		if base == "" {
			base = fmt.Sprintf("File %010d", f.Offset/512)
		}
		name := base + "." + f.Ext
		key := dir.Path() + "/" + strings.ToLower(name)
		if n := names[key]; n > 0 {
			name = fmt.Sprintf("%s (%d).%s", base, n+1, f.Ext)
		}
		names[key]++
		e := &fsys.Entry{
			ID:     uint64(f.Offset),
			Name:   name,
			Size:   f.Length,
			State:  fsys.Carved,
			Health: 100,
			Runs:   []fsys.Run{{Offset: f.Offset, Length: f.Length}},
		}
		if f.Partial {
			e.Health = 60
			e.Note = "The end of the file couldn't be confirmed; it may be cut short or fragmented"
		}
		dir.Add(e)
	}
	v.Sort()
	return v
}
