package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/cleanup"
	"github.com/xmhrnx/remanance/engine/devsync"
	"github.com/xmhrnx/remanance/engine/dupes"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/rename"
	"github.com/xmhrnx/remanance/engine/space"
	"github.com/xmhrnx/remanance/engine/trash"
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

// runRename renames files by a pattern, showing the new names first
// unless -yes is given.
func runRename(args []string) error {
	fs := flag.NewFlagSet("rename", flag.ExitOnError)
	var r rename.Rules
	fs.StringVar(&r.Pattern, "pattern", "{name}", "the new name: {name} {ext} {n:3} {date:2006-01-02} {parent}")
	fs.IntVar(&r.Start, "start", 1, "where {n} starts")
	fs.IntVar(&r.Step, "step", 1, "how much {n} counts up by")
	fs.StringVar(&r.Find, "find", "", "text to replace")
	fs.StringVar(&r.Replace, "replace", "", "what replaces it")
	fs.BoolVar(&r.Regexp, "regexp", false, "-find is a regular expression")
	fs.StringVar(&r.Ext, "ext", "", "a new extension")
	fs.BoolVar(&r.LowerExt, "lower-ext", false, "lowercase extensions")
	cs := fs.String("case", "", "lower, upper, title or sentence")
	yes := fs.Bool("yes", false, "rename, rather than only show the new names")
	fs.Parse(args)
	if fs.NArg() == 0 {
		return errors.New("usage: datadoc-cli rename -pattern P [-find X -replace Y] [-yes] FILE...")
	}
	switch strings.ToLower(*cs) {
	case "lower":
		r.Case = rename.CaseLower
	case "upper":
		r.Case = rename.CaseUpper
	case "title":
		r.Case = rename.CaseTitle
	case "sentence":
		r.Case = rename.CaseSentence
	}
	paths := fs.Args()
	sort.Strings(paths)
	plan, err := rename.Plan(paths, r)
	if err != nil {
		return err
	}
	for _, c := range plan {
		if c.Same() {
			continue
		}
		line := fmt.Sprintf("%s  ->  %s", filepath.Base(c.Path), c.New)
		if c.Problem != "" {
			line += "   (" + c.Problem + ")"
		}
		fmt.Println(line)
	}
	if !*yes {
		fmt.Println("\nNothing renamed yet: run again with -yes.")
		return nil
	}
	done, err := rename.Apply(plan)
	fmt.Printf("%d files renamed\n", len(done))
	return err
}

// runSync runs or sets up syncing with the user's other devices.
func runSync(args []string) error {
	const syncUsage = `usage:
  datadoc-cli sync id                     this device's ID
  datadoc-cli sync add DEVICE-ID [ADDR]   add one of your devices
  datadoc-cli sync share FOLDER [-mode two-way|send|receive]
                                            sync a folder with every device added
  datadoc-cli sync run                    sync until stopped`
	if len(args) == 0 {
		return errors.New(syncUsage)
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	host, _ := os.Hostname()
	n, err := devsync.Open(filepath.Join(cfg, "DataDoc", "sync"), host)
	if err != nil {
		return err
	}
	defer n.Close()
	switch args[0] {
	case "id":
		fmt.Println(n.ID())
	case "add":
		if len(args) < 2 {
			return errors.New(syncUsage)
		}
		addr := ""
		if len(args) > 2 {
			addr = args[2]
		}
		if err := n.AddDevice(args[1], "", addr); err != nil {
			return err
		}
		fmt.Println("Added. Accept this device on the other one too:", n.ID())
	case "share":
		fs := flag.NewFlagSet("share", flag.ExitOnError)
		mode := fs.String("mode", "two-way", "two-way, send or receive")
		fs.Parse(args[1:])
		if fs.NArg() != 1 {
			return errors.New(syncUsage)
		}
		dir, err := filepath.Abs(fs.Arg(0))
		if err != nil {
			return err
		}
		f := devsync.Folder{Label: filepath.Base(dir), Path: dir, Mode: map[string]devsync.Mode{"send": devsync.SendOnly, "receive": devsync.ReceiveOnly}[*mode]}
		for _, d := range n.Config().Devices {
			f.Devices = append(f.Devices, d.ID)
		}
		if _, err := n.AddFolder(f); err != nil {
			return err
		}
		fmt.Printf("Sharing %s with %d devices. Run \"datadoc-cli sync run\" to sync.\n", dir, len(f.Devices))
	case "run":
		return syncRun(n)
	default:
		return errors.New(syncUsage)
	}
	return nil
}

// syncRun syncs until interrupted, printing what changes and accepting
// nothing on its own: requests are shown for "sync add" to answer.
func syncRun(n *devsync.Node) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := n.Start(ctx, devsync.Options{Discovery: true}); err != nil {
		return err
	}
	fmt.Printf("This device: %s\nListening on %s. Stop with Ctrl+C.\n", n.ID(), n.Addr())
	last := ""
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		st := n.Status()
		var b strings.Builder
		for _, d := range st.Devices {
			state := "not connected"
			if d.Connected {
				state = "connected " + d.Addr
			}
			fmt.Fprintf(&b, "device  %s (%s): %s\n", d.Name, d.ID[:9], state)
		}
		for _, f := range st.Folders {
			fmt.Fprintf(&b, "folder  %s: %s, %d files, %d to get\n", f.Label, f.State.Label(), f.Files, f.Need)
		}
		for _, p := range st.Pending {
			if p.Folder == "" {
				fmt.Fprintf(&b, "request %s wants to connect: datadoc-cli sync add %s\n", p.Name, p.Device)
			} else {
				fmt.Fprintf(&b, "offer   %s shares %q: accept it in the app\n", p.Name, p.Label)
			}
		}
		if s := b.String(); s != last {
			fmt.Printf("\n%s", s)
			last = s
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}
