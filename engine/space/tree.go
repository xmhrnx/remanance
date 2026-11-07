package space

import "sort"

// KindTotal is how much one kind of file takes.
type KindTotal struct {
	Kind  Kind
	Size  int64
	Files int
}

// total adds up every folder from its children, and sorts them largest
// first.
func total(n *Node) {
	if !n.IsDir {
		return
	}
	n.Size, n.Files = 0, 0
	for _, c := range n.Children {
		total(c)
		n.Size += c.Size
		n.Files += c.Files
	}
	sort.Slice(n.Children, func(i, j int) bool {
		if n.Children[i].Size != n.Children[j].Size {
			return n.Children[i].Size > n.Children[j].Size
		}
		return n.Children[i].Name < n.Children[j].Name
	})
}

// Trail is the folders from the top down to n.
func (n *Node) Trail() []*Node {
	var out []*Node
	for x := n; x != nil; x = x.Parent {
		out = append([]*Node{x}, out...)
	}
	return out
}

// Remove takes n out of the tree after it was deleted, and takes its
// size off every folder above it.
func (n *Node) Remove() {
	p := n.Parent
	if p == nil {
		return
	}
	for i, c := range p.Children {
		if c == n {
			p.Children = append(p.Children[:i], p.Children[i+1:]...)
			break
		}
	}
	for x := p; x != nil; x = x.Parent {
		x.Size -= n.Size
		x.Files -= n.Files
	}
}

// Kinds sums the files below n by what they are, largest first.
func (n *Node) Kinds() []KindTotal {
	sums := map[Kind]*KindTotal{}
	var walk func(*Node)
	walk = func(x *Node) {
		if !x.IsDir {
			k := KindOf(x.Name)
			t := sums[k]
			if t == nil {
				t = &KindTotal{Kind: k}
				sums[k] = t
			}
			t.Size += x.Size
			t.Files++
			return
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	walk(n)
	out := make([]KindTotal, 0, len(sums))
	for _, t := range sums {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}
