package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/xmhrnx/remanance/engine/dupes"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/trash"
)

// runDupes lists duplicate files, and with -remove moves all but the
// suggested keeper of each group to the trash.
func runDupes(args []string) error {
	fs := flag.NewFlagSet("dupes", flag.ExitOnError)
	similar := fs.Bool("similar", false, "also find photos and videos that look the same")
	threshold := fs.Int("threshold", 6, "how different similar photos may be, 0 to 12")
	minSize := fs.Int64("min", 1, "skip files smaller than this many bytes")
	remove := fs.Bool("remove", false, "move all but one exact copy of each file to the trash")
	fs.Parse(args)
	if fs.NArg() == 0 {
		return errors.New("usage: datadoc-cli dupes [-similar] [-remove] FOLDER...")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	groups, err := dupes.Find(ctx, fs.Args(), dupes.Options{Similar: *similar, Threshold: *threshold, MinSize: *minSize},
		func(p dupes.Progress) {
			fmt.Fprintf(os.Stderr, "\r%-24s %d/%d, %d groups   ", p.Phase.Label(), p.Done, p.Of, p.Groups)
		})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	var wasted int64
	removed := 0
	for _, g := range groups {
		wasted += g.Wasted()
		fmt.Printf("%s, %s to gain\n", g.Kind.Label(), recovery.FormatBytes(g.Wasted()))
		for _, f := range g.Files {
			mark := " "
			if f.Keep {
				mark = "*"
			}
			fmt.Printf("  %s %10s  %s\n", mark, recovery.FormatBytes(f.Size), f.Path)
			// Only exact copies are removed: similar photos need a person
			// to choose.
			if *remove && !f.Keep && g.Kind == dupes.Exact {
				if err := trash.Remove(f.Path, trash.Supported); err != nil {
					fmt.Fprintln(os.Stderr, "  could not remove:", err)
				} else {
					removed++
				}
			}
		}
		fmt.Println()
	}
	fmt.Printf("%d groups, %s taken by copies (* is the one to keep)\n", len(groups), recovery.FormatBytes(wasted))
	if *remove {
		fmt.Printf("%d copies removed\n", removed)
	}
	return nil
}
