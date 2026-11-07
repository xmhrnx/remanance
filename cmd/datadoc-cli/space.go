package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"

	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/space"
)

// runSpace prints the largest folders and files under a folder.
func runSpace(args []string) error {
	fs := flag.NewFlagSet("space", flag.ExitOnError)
	top := fs.Int("top", 20, "how many to list")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("usage: datadoc-cli space [-top N] FOLDER")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	root, err := space.Measure(ctx, fs.Arg(0), func(p space.Progress) {
		fmt.Fprintf(os.Stderr, "\r%d files, %s   ", p.Files, recovery.FormatBytes(p.Bytes))
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	fmt.Printf("%s  %s in %d files\n\n", root.Path, recovery.FormatBytes(root.Size), root.Files)
	var all []*space.Node
	var walk func(n *space.Node)
	walk = func(n *space.Node) {
		for _, c := range n.Children {
			all = append(all, c)
			walk(c)
		}
	}
	walk(root)
	print := func(title string, keep func(*space.Node) bool) {
		var ns []*space.Node
		for _, n := range all {
			if keep(n) {
				ns = append(ns, n)
			}
		}
		sort.Slice(ns, func(i, j int) bool { return ns[i].Size > ns[j].Size })
		if len(ns) > *top {
			ns = ns[:*top]
		}
		fmt.Println(title)
		for _, n := range ns {
			fmt.Printf("  %10s  %s\n", recovery.FormatBytes(n.Size), n.Path)
		}
		fmt.Println()
	}
	// Folders directly under the root, then the largest files anywhere.
	print("Folders", func(n *space.Node) bool { return n.IsDir && n.Parent == root })
	print("Largest files", func(n *space.Node) bool { return !n.IsDir })
	fmt.Println("By kind")
	for _, k := range root.Kinds() {
		fmt.Printf("  %10s  %s\n", recovery.FormatBytes(k.Size), k.Kind.Label())
	}
	return nil
}
