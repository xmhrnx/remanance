package model

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/locales"
)

// Kind tells the tree's top-level nodes apart.
type Kind int

const (
	KindFolder Kind = iota
	KindVolume
	KindLost
	KindCarved
)

// Node is a folder in the results tree: a volume, its lost-and-found, the
// files found by signature, or a folder inside one of them.
type Node struct {
	Entry  *fsys.Entry
	Kind   Kind
	Title  string // the name shown; a volume's describes it
	Detail string // a volume's file system and size
	Volume *scan.VolumeResult
	Lost   bool // a partition no table describes any more
	Parent *Node
	kids   []giode.TreeNode
	built  bool
}

func (n *Node) Key() any { return n }

func (n *Node) HasChildren() bool {
	for _, c := range n.Entry.Children {
		if c.IsDir {
			return true
		}
	}
	return false
}

func (n *Node) Children() []giode.TreeNode {
	if !n.built {
		n.built = true
		for _, c := range n.Entry.Children {
			if c.IsDir {
				n.kids = append(n.kids, &Node{Entry: c, Kind: KindFolder, Title: c.Name, Parent: n})
			}
		}
	}
	return n.kids
}

// Child finds the tree node of one of this folder's subfolders.
func (n *Node) Child(e *fsys.Entry) *Node {
	for _, k := range n.Children() {
		if c := k.(*Node); c.Entry == e {
			return c
		}
	}
	return nil
}

// Trail is the path from the top of the tree to this node.
func (n *Node) Trail() []*Node {
	var out []*Node
	for c := n; c != nil; c = c.Parent {
		out = append([]*Node{c}, out...)
	}
	return out
}

// roots builds the top of the results tree.
func roots(res *scan.Result) []*Node {
	var out []*Node
	for _, v := range res.Volumes {
		if v.Volume == nil {
			continue
		}
		p := v.Partition
		label := v.Volume.Label
		if label == "" {
			label = p.Label
		}
		title := label
		if title == "" {
			title = locales.T("node.partition", "n", p.Index)
		}
		detail := locales.T("node.volume_detail", "fs", v.Volume.FS, "size", bytes(p.Length))
		vol := &Node{Entry: v.Volume.Root, Kind: KindVolume, Title: title, Detail: detail, Volume: v, Lost: p.Lost}
		out = append(out, vol)
		if v.Volume.Lost != nil && len(v.Volume.Lost.Children) > 0 {
			out = append(out, &Node{Entry: v.Volume.Lost, Kind: KindLost, Title: locales.T("node.lost_and_found"), Detail: title, Volume: v})
		}
	}
	if res.Carved != nil && len(res.Carved.Root.Children) > 0 {
		out = append(out, &Node{Entry: res.Carved.Root, Kind: KindCarved, Title: locales.T("node.found_by_signature"),
			Detail: locales.N("node.files", res.Carved.Stats().Files)})
	}
	return out
}

// Prefix is the folder a node's files go into when recovered, so files
// from different volumes don't mix.
func (n *Node) prefix() string {
	top := n.Trail()[0]
	// These folders keep their English names in every language, so a
	// recovery lays its files out the same wherever it's run.
	switch top.Kind {
	case KindCarved:
		return "Found by signature"
	case KindLost:
		return top.Detail + "/Lost and found"
	}
	return top.Title
}
