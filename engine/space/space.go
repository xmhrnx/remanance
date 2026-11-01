// Package space measures what fills a drive or folder: every folder's
// size, counted from the files below it, ready to draw as a treemap
// (see Layout).
package space

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
)

// Node is a folder or file and what it holds.
type Node struct {
	Name     string
	Path     string // on this computer
	Size     int64  // bytes, everything below included
	Files    int    // files below
	IsDir    bool
	Children []*Node // largest first
	Parent   *Node
	// Denied is set on a folder that couldn't be read.
	Denied bool
}

// Progress says how a measurement is going.
type Progress struct {
	Files int64
	Bytes int64
	Path  string // a folder being read
}

// Measure walks root and sizes every folder below it. Folders on other
// drives mounted inside it are left out, and links aren't followed, so
// nothing is counted twice.
func Measure(ctx context.Context, root string, progress func(Progress)) (*Node, error) {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	w := &walker{ctx: ctx, dev: deviceOf(info), reads: make(chan struct{}, runtime.NumCPU()*2), spawn: make(chan struct{}, 64)}
	top := &Node{Name: displayName(root), Path: root, IsDir: info.IsDir()}
	if !top.IsDir {
		top.Size, top.Files = info.Size(), 1
		return top, nil
	}
	stop := make(chan struct{})
	if progress != nil {
		go w.report(progress, stop)
	}
	w.dir(top)
	w.wg.Wait()
	close(stop)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	total(top)
	if progress != nil {
		progress(Progress{Files: w.files.Load(), Bytes: w.bytes.Load()})
	}
	return top, nil
}

type walker struct {
	ctx          context.Context
	dev          uint64
	reads        chan struct{} // folders read at once
	spawn        chan struct{} // goroutines reading folders
	wg           sync.WaitGroup
	files, bytes atomic.Int64
	current      atomic.Pointer[string]
}

// dir reads a folder and its children's folders, each on its own
// goroutine while there's room, so a drive with deep trees and slow
// reads still keeps every core and the disk's queue busy.
func (w *walker) dir(n *Node) {
	if w.ctx.Err() != nil {
		return
	}
	w.reads <- struct{}{}
	w.current.Store(&n.Path)
	entries, err := os.ReadDir(n.Path)
	<-w.reads
	if err != nil {
		n.Denied = true
	}
	n.Children = make([]*Node, 0, len(entries))
	for _, e := range entries {
		c := &Node{Name: e.Name(), Path: filepath.Join(n.Path, e.Name()), Parent: n}
		t := e.Type()
		switch {
		case t&fs.ModeSymlink != 0, t&(fs.ModeDevice|fs.ModeNamedPipe|fs.ModeSocket|fs.ModeIrregular) != 0:
			continue
		case e.IsDir():
			info, err := e.Info()
			if err != nil || (w.dev != 0 && deviceOf(info) != w.dev) {
				continue // another drive mounted here
			}
			c.IsDir = true
			n.Children = append(n.Children, c)
			select {
			case w.spawn <- struct{}{}:
				w.wg.Add(1)
				go func() {
					defer w.wg.Done()
					w.dir(c)
					<-w.spawn
				}()
			default:
				w.dir(c)
			}
		default:
			info, err := e.Info()
			if err != nil {
				continue
			}
			c.Size, c.Files = allocated(info), 1
			w.files.Add(1)
			w.bytes.Add(c.Size)
			n.Children = append(n.Children, c)
		}
	}
}

func (w *walker) report(progress func(Progress), stop chan struct{}) {
	t := newTicker()
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			p := Progress{Files: w.files.Load(), Bytes: w.bytes.Load()}
			if c := w.current.Load(); c != nil {
				p.Path = *c
			}
			progress(p)
		}
	}
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

func displayName(p string) string {
	if b := filepath.Base(p); b != "" && b != "." && b != string(filepath.Separator) {
		return b
	}
	return p
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

// KindTotal is how much one kind of file takes.
type KindTotal struct {
	Kind  Kind
	Size  int64
	Files int
}
