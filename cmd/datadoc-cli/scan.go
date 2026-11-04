package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/scan"
)

// scanSource opens and scans a source, or with folder set the drive it's
// on for what was in it, printing progress on one line.
func scanSource(arg, folder string, deep bool) (*scan.Result, disk.Device, error) {
	var dev disk.Device
	var within string
	var err error
	if folder != "" {
		dev, within, err = openFolder(folder)
	} else {
		dev, err = openSource(arg)
	}
	if err != nil {
		return nil, nil, err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	mode := scan.Quick
	if deep {
		mode = scan.Deep
	}
	res, err := scan.Run(ctx, dev, scan.Options{Mode: mode, Folder: within}, func(p scan.Progress) {
		line := fmt.Sprintf("\r%-30s %5.1f%%  files %d, deleted %d", p.Phase, p.Fraction*100, p.Files, p.Deleted)
		if p.Phase == scan.PhaseSectors {
			line += fmt.Sprintf(", by signature %d  %s/s, %s left", p.Carved, recovery.FormatBytes(int64(p.Speed)), p.Remaining.Round(time.Second))
		}
		fmt.Fprintf(os.Stderr, "%-120s", line)
	})
	fmt.Fprintln(os.Stderr)
	if err != nil && res == nil {
		dev.Close()
		return nil, nil, err
	}
	return res, dev, err
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	deep := fs.Bool("deep", false, "read every sector: lost partitions and files found by signature")
	in := fs.String("in", "", "only what was deleted from this folder, instead of a SOURCE")
	fs.Parse(args)
	if fs.NArg() != 1 && !(*in != "" && fs.NArg() == 0) {
		return fmt.Errorf("scan needs one SOURCE, or -in FOLDER")
	}
	res, dev, err := scanSource(fs.Arg(0), *in, *deep)
	if dev != nil {
		defer dev.Close()
	}
	if res == nil {
		return err
	}
	printResult(res)
	return err
}

func printResult(res *scan.Result) {
	if res.Files {
		fmt.Printf("%s (%s of files)\n", res.Device.Name, recovery.FormatBytes(res.Device.Size))
	} else {
		fmt.Printf("%s (%s), partition table: %s", res.Device.Name, recovery.FormatBytes(res.Device.Size), res.Table.Scheme)
		if res.Table.Damaged {
			fmt.Print(" (damaged)")
		}
		fmt.Println()
	}
	for _, v := range res.Volumes {
		p := v.Partition
		label := p.Label
		if v.Volume != nil && v.Volume.Label != "" {
			label = v.Volume.Label
		}
		lost := ""
		if p.Lost {
			lost = "  LOST"
		}
		fmt.Printf("  #%d  %-6s %-16q at %-10d %9s%s\n", p.Index, p.FS, label, p.Offset, recovery.FormatBytes(p.Length), lost)
		if v.Err != nil {
			fmt.Printf("       %v\n", v.Err)
		}
		if v.Volume != nil {
			s := v.Volume.Stats()
			fmt.Printf("       %d files, %d deleted (%s)\n", s.Files, s.Deleted, recovery.FormatBytes(s.DeletedBytes))
			for _, w := range v.Volume.Warnings {
				fmt.Printf("       note: %s\n", w)
			}
		}
	}
	if res.Carved != nil {
		fmt.Printf("  Found by signature: %d files\n", res.Carved.Stats().Files)
		for _, cat := range res.Carved.Root.Children {
			for _, typ := range cat.Children {
				fmt.Printf("       %-10s %-22s %d\n", cat.Name, typ.Name, len(typ.Children))
			}
		}
	}
	if len(res.Bad) > 0 {
		var n int64
		for _, b := range res.Bad {
			n += b.Length
		}
		fmt.Printf("  Unreadable: %s in %d places\n", recovery.FormatBytes(n), len(res.Bad))
	}
	fmt.Printf("Scanned in %s.\n", res.Duration.Round(time.Millisecond))
}
