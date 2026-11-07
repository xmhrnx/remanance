package space

import (
	"io/fs"
	"os"
	"path/filepath"
)

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

func displayName(p string) string {
	if b := filepath.Base(p); b != "" && b != "." && b != string(filepath.Separator) {
		return b
	}
	return p
}
