package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/imaging"
	"github.com/xmhrnx/remanance/engine/recovery"
)

func runImage(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("image needs SOURCE and OUTPUT")
	}
	dev, err := openSource(args[0])
	if err != nil {
		return err
	}
	defer dev.Close()
	if dev.Info().Kind == disk.KindFiles {
		return fmt.Errorf("%s is a set of files, not a drive: scan or recover from it directly", args[0])
	}
	if recovery.SameDevice(dev.Info(), args[1]) {
		return fmt.Errorf("%s is on the drive being imaged; choose another drive", args[1])
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := imaging.Create(ctx, dev, args[1], func(p imaging.Progress) {
		fmt.Fprintf(os.Stderr, "\r%5.1f%%  %s of %s  %s/s  unreadable %s   ", float64(p.Done)*100/float64(p.Total),
			recovery.FormatBytes(p.Done), recovery.FormatBytes(p.Total), recovery.FormatBytes(int64(p.Speed)), recovery.FormatBytes(p.BadBytes))
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	fmt.Printf("Wrote %s (%s) in %s; %d unreadable ranges.\n", res.Path, recovery.FormatBytes(res.Size), res.Duration.Round(time.Second), len(res.Bad))
	return nil
}
