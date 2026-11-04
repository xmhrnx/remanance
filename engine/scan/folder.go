package scan

import (
	"errors"
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// ErrNoFolder means the folder to scan isn't in the volume's file
// system, such as when it's been deleted since it was chosen.
var ErrNoFolder = errors.New("the folder isn't on this drive any more")

// keepFolder narrows v to the folder at path, slash-separated from the
// volume's top, leaving out the rest and the files whose folder isn't
// known. The folder's entry keeps its parents, so paths stay whole.
func keepFolder(v *fsys.Volume, path string) error {
	e := v.Root
	for _, name := range strings.Split(strings.Trim(path, "/"), "/") {
		if name == "" {
			continue
		}
		if e = child(e, name); e == nil {
			return ErrNoFolder
		}
	}
	v.Root = e
	v.Lost = &fsys.Entry{Name: v.Lost.Name, IsDir: true, State: fsys.Orphan, Health: 100}
	return nil
}

// child is the folder in e with the name given: the one that still
// exists over a deleted one of the same name, and matched whatever its
// case where no name matches exactly, as Windows' file systems do.
func child(e *fsys.Entry, name string) *fsys.Entry {
	var fold *fsys.Entry
	for _, c := range e.Children {
		if !c.IsDir {
			continue
		}
		switch {
		case c.Name == name && c.State == fsys.Existing:
			return c
		case strings.EqualFold(c.Name, name) && (fold == nil || fold.State != fsys.Existing):
			fold = c
		}
	}
	return fold
}
