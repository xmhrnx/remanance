package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/xmhrnx/remanance/engine/iosbackup"
	"github.com/xmhrnx/remanance/engine/iphone"
	"github.com/xmhrnx/remanance/engine/recovery"
)

// runBackup backs up an iPhone connected by USB, for scan and recover
// to read.
func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	to := fs.String("to", iosbackup.OwnPlace(), "the folder to keep the backup in")
	fs.Parse(args)
	phones, err := iphone.Connected()
	if err != nil {
		return err
	}
	if len(phones) == 0 {
		return fmt.Errorf("no iPhone or iPad is connected by USB")
	}
	p := phones[0]
	if fs.NArg() == 1 {
		p = iphone.Phone{}
		for _, q := range phones {
			if q.UDID == fs.Arg(0) {
				p = q
			}
		}
		if p.UDID == "" {
			return fmt.Errorf("no iPhone %s is connected", fs.Arg(0))
		}
	} else if len(phones) > 1 {
		return fmt.Errorf("%d iPhones are connected: name one by the identifier \"list\" shows", len(phones))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	dir, err := iphone.Backup(ctx, p, *to, func(pr iphone.Progress) {
		line := pr.Step
		if pr.Fraction > 0 {
			line += fmt.Sprintf("  %5.1f%%", pr.Fraction*100)
		}
		if pr.Bytes > 0 {
			line += "  " + recovery.FormatBytes(pr.Bytes)
		}
		fmt.Fprintf(os.Stderr, "\r%-100s", line)
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	fmt.Printf("Backed up %s to %s\n", p.Name, dir)
	return nil
}
