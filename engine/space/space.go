// Package space measures what fills a drive or folder: every folder's
// size, counted from the files below it, ready to draw as a treemap
// (see Layout).
package space

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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

// walker reads the tree, keeping count as it goes.
type walker struct {
	ctx          context.Context
	dev          uint64
	reads        chan struct{} // folders read at once
	spawn        chan struct{} // goroutines reading folders
	wg           sync.WaitGroup
	files, bytes atomic.Int64
	current      atomic.Pointer[string]
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
