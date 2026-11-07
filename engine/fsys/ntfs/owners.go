package ntfs

import (
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
)

// ownsChild checks the sequence number in a file's parent reference: a
// parent record reused for something else since isn't its folder. A
// deleted folder's sequence number went up by one when it was deleted.
func ownsChild(parent, child *node) bool {
	want := child.name.parentSeq
	have := parent.rec.seq
	if want == 0 || have == want {
		return true
	}
	return !parent.rec.inUse() && have == want+1
}

func createsCycle(parent, child *fsys.Entry) bool {
	for p, depth := parent, 0; p != nil && depth < 4096; p, depth = p.Parent, depth+1 {
		if p == child {
			return true
		}
	}
	return false
}

// hideSystem leaves out NTFS's metadata files ($Extend and what's in it).
func hideSystem(n *node, nodes []*node) bool {
	if n.name.parent == recordExtend || n.rec.number == recordExtend {
		return true
	}
	return n.name.parent == recordRoot && strings.HasPrefix(n.name.name, "$") && n.rec.number < 64
}
