package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/xmhrnx/remanance/engine/cleanup"
	"github.com/xmhrnx/remanance/engine/recovery"
)

// runClean lists what can be cleaned to free space, and with -yes
// cleans it: the kinds that are safe to clean unseen, or every kind with
// -all.
func runClean(args []string) error {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	all := fs.Bool("all", false, "also the trash, developer caches, installers and old downloads")
	yes := fs.Bool("yes", false, "clean, rather than only list")
	fs.Parse(args)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if s, err := cleanup.Free(cleanup.Drive()); err == nil {
		fmt.Printf("%s: %s free of %s\n\n", cleanup.Drive(), recovery.FormatBytes(s.Free), recovery.FormatBytes(s.Total))
	}
	groups, err := cleanup.Find(ctx, func(p cleanup.Progress) {
		fmt.Fprintf(os.Stderr, "\r%s: %d files, %s   ", p.Kind.Label(), p.Files, recovery.FormatBytes(p.Bytes))
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	var items []cleanup.Item
	var size int64
	for _, g := range groups {
		mark := " "
		if *all || g.Kind.Safe() {
			mark = "*"
			items = append(items, g.Items...)
			size += g.Size()
		}
		fmt.Printf("%s %10s  %s (%d files)\n", mark, recovery.FormatBytes(g.Size()), g.Kind.Label(), g.Files())
		for i, it := range g.Items {
			if i == 5 {
				fmt.Printf("  %10s  and %d more\n", "", len(g.Items)-i)
				break
			}
			fmt.Printf("  %10s  %s\n", recovery.FormatBytes(it.Size), it.Path)
		}
	}
	if !*yes {
		fmt.Printf("\n%s marked *; run again with -yes to clean it.\n", recovery.FormatBytes(size))
		return nil
	}
	r, err := cleanup.Clean(ctx, items, nil)
	if err != nil {
		return err
	}
	fmt.Printf("\nFreed %s from %d files.", recovery.FormatBytes(r.Freed), r.Files)
	if r.Trashed > 0 {
		fmt.Printf(" %s went to the trash.", recovery.FormatBytes(r.Trashed))
	}
	if r.Left > 0 {
		fmt.Printf(" %d files were in use and were left.", r.Left)
	}
	fmt.Println()
	return nil
}
