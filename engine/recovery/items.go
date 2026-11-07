package recovery

import (
	"path"
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// relPath keeps the entry's folders below the item's base.
func relPath(e *fsys.Entry, it Item) string {
	p := e.Path()
	if it.Base != "" && it.Base != "/" {
		p = strings.TrimPrefix(p, it.Base)
	}
	var parts []string
	for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
		if s != "" {
			parts = append(parts, SafeName(s))
		}
	}
	if len(parts) == 0 {
		return SafeName(e.Name)
	}
	return path.Join(parts...)
}
