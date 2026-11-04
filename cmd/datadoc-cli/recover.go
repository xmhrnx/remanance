package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/recovery"
)

func runRecover(args []string) error {
	fs := flag.NewFlagSet("recover", flag.ExitOnError)
	to := fs.String("to", "", "folder to copy recovered files into (required)")
	deep := fs.Bool("deep", false, "deep scan first: lost partitions and files found by signature")
	all := fs.Bool("all", false, "recover existing files too, not only deleted ones")
	flat := fs.Bool("flat", false, "put every file straight into the folder")
	types := fs.String("type", "", "only these extensions, comma separated (jpg,png,pdf)")
	in := fs.String("in", "", "only what was deleted from this folder, instead of a SOURCE")
	fs.Parse(args)
	if *to == "" || fs.NArg() != 1 && !(*in != "" && fs.NArg() == 0) {
		return fmt.Errorf("recover needs -to DIR and one SOURCE, or -in FOLDER")
	}
	res, dev, err := scanSource(fs.Arg(0), *in, *deep)
	if dev != nil {
		defer dev.Close()
	}
	if res == nil {
		return err
	}
	if recovery.SameDevice(res.Device, *to) {
		return fmt.Errorf("%s is on the drive being recovered; writing there could overwrite the files. Choose another drive", *to)
	}
	want := map[string]bool{}
	for _, t := range strings.Split(*types, ",") {
		if t = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), ".")); t != "" {
			want[t] = true
		}
	}
	keep := func(e *fsys.Entry) bool {
		if !*all && e.State == fsys.Existing {
			return false
		}
		return len(want) == 0 || want[e.Ext()]
	}

	var items []recovery.Item
	collect := func(root *fsys.Entry, prefix string) {
		root.Walk(func(e *fsys.Entry) bool {
			if !e.IsDir && keep(e) {
				items = append(items, recovery.Item{Entry: e, Prefix: prefix, Base: ""})
			}
			return true
		})
	}
	for _, v := range res.Volumes {
		if v.Volume == nil {
			continue
		}
		name := fmt.Sprintf("Partition %d %s", v.Partition.Index, v.Volume.FS)
		if v.Volume.Label != "" {
			name += " " + v.Volume.Label
		}
		if res.Files {
			name = v.Volume.Label
		}
		collect(v.Volume.Root, name)
		collect(v.Volume.Lost, name+"/Lost and found")
	}
	if res.Carved != nil {
		collect(res.Carved.Root, "Found by signature")
	}
	if len(items) == 0 {
		fmt.Println("Nothing to recover.")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rep, err := recovery.Recover(ctx, dev, items, recovery.DirSink{Dir: *to}, recovery.Options{KeepFolders: !*flat, Report: true}, func(p recovery.Progress) {
		fmt.Fprintf(os.Stderr, "\rRecovering %d/%d  %s of %s  %-40.40s", p.Files, p.TotalFiles, recovery.FormatBytes(p.Done), recovery.FormatBytes(p.Total), p.Current)
	})
	fmt.Fprintln(os.Stderr)
	if rep != nil {
		fmt.Printf("Recovered %d files (%s) into %s; %d failed.\n", rep.Recovered, recovery.FormatBytes(rep.Bytes), rep.Location, rep.Failed)
	}
	return err
}
