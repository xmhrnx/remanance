// Command remanence-cli is the recovery engine without the interface: for
// scripts, servers and machines without a display.
//
//	remanence-cli list
//	remanence-cli scan [-deep] DEVICE|IMAGE
//	remanence-cli recover [-deep] [-all] [-flat] [-type jpg,png] -to DIR DEVICE|IMAGE
//	remanence-cli image DEVICE OUTPUT.img
package main

import (
	"fmt"
	"os"

	"github.com/xmhrnx/remanance/ui/platform/iosbackups"
)

const usage = `Remanence recovers deleted and lost files.

Usage:
  remanence-cli list                         drives this computer can see
  remanence-cli scan [-deep] SOURCE          what can be recovered
  remanence-cli recover -to DIR [-deep] [-all] [-flat] [-type jpg,pdf] SOURCE
                                             copy deleted files out (-all: every file)
  remanence-cli image SOURCE OUTPUT.img      copy a whole drive to an image first
  remanence-cli backup [-to DIR] [UDID]      back up an iPhone connected by USB, to scan it

SOURCE is a drive from "list" (\\.\PhysicalDrive1, /dev/sdb, /dev/rdisk2),
a disk image file, or an iPhone backup's folder ("list" shows the ones
Finder and iTunes made). Reading drives needs administrator rights. For
an encrypted backup, set REMANENCE_BACKUP_PASSWORD to its password.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	iosbackups.Register()
	var err error
	switch os.Args[1] {
	case "list":
		err = runList()
	case "scan":
		err = runScan(os.Args[2:])
	case "recover":
		err = runRecover(os.Args[2:])
	case "image":
		err = runImage(os.Args[2:])
	case "backup":
		err = runBackup(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
