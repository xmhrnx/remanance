package records

import (
	"fmt"
	"path"
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// unique numbers a file's name when dir has one by that name, which two
// databases of the same app can give: "Calls (2).csv".
func unique(dir *fsys.Entry, name string) string {
	taken := func(n string) bool {
		for _, c := range dir.Children {
			if strings.EqualFold(c.Name, n) {
				return true
			}
		}
		return false
	}
	ext := path.Ext(name)
	out := name
	for i := 2; taken(out); i++ {
		out = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), i, ext)
	}
	return out
}

// wal finds the database's write-ahead log beside it.
func wal(e *fsys.Entry) *fsys.Entry {
	if e.Parent == nil {
		return nil
	}
	for _, c := range e.Parent.Children {
		if c.Name == e.Name+"-wal" && c.State == fsys.Existing && c.Size > 32 {
			return c
		}
	}
	return nil
}

// folder finds or adds the folder named name in dir.
func folder(dir *fsys.Entry, name string) *fsys.Entry {
	for _, c := range dir.Children {
		if c.IsDir && c.Name == name {
			return c
		}
	}
	f := &fsys.Entry{Name: name, IsDir: true, Health: 100, State: fsys.Deleted}
	dir.Add(f)
	return f
}
