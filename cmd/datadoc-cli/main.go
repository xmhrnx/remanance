// Command datadoc-cli is the recovery engine without the interface: for
// scripts, servers and machines without a display.
//
//	datadoc-cli list
//	datadoc-cli scan [-deep] DEVICE|IMAGE
//	datadoc-cli recover [-deep] [-all] [-flat] [-type jpg,png] -to DIR DEVICE|IMAGE
//	datadoc-cli image DEVICE OUTPUT.img
//	datadoc-cli space FOLDER
//	datadoc-cli clean [-all] [-yes]
//	datadoc-cli dupes [-similar] [-remove] FOLDER...
//	datadoc-cli rename -pattern P [-yes] FILE...
package main

import (
	"fmt"
	"os"

	"github.com/xmhrnx/remanance/ui/platform/adbphones"
	"github.com/xmhrnx/remanance/ui/platform/iosbackups"
	"github.com/xmhrnx/remanance/ui/platform/iphones"
)

const usage = `DataDoc recovers deleted and lost files.

Usage:
  datadoc-cli list                         drives this computer can see
  datadoc-cli scan [-deep] SOURCE          what can be recovered
  datadoc-cli recover -to DIR [-deep] [-all] [-flat] [-type jpg,pdf] SOURCE
                                             copy deleted files out (-all: every file)
  datadoc-cli image SOURCE OUTPUT.img      copy a whole drive to an image first

Tools:
  datadoc-cli space [-top N] FOLDER        what fills a drive or folder
  datadoc-cli clean [-all] [-yes]          temporary files, caches and old downloads
                                             to clean (-yes cleans the safe ones)
  datadoc-cli dupes [-similar] [-remove] FOLDER...
                                             duplicate files and similar photos
  datadoc-cli rename -pattern "{date} {n:3}" [-find X -replace Y] [-case lower] [-yes] FILE...
                                             rename many files at once

SOURCE is a drive from "list" (\\.\PhysicalDrive1, /dev/sdb, /dev/rdisk2),
a disk image file, an iPhone or Android phone connected by USB, or an
iPhone backup's folder ("list" shows them all). Reading drives needs administrator rights. For
an encrypted backup, set DATADOC_BACKUP_PASSWORD to its password.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	iosbackups.Register()
	adbphones.Register()
	iphones.Register()
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
	case "space":
		err = runSpace(os.Args[2:])
	case "clean":
		err = runClean(os.Args[2:])
	case "dupes":
		err = runDupes(os.Args[2:])
	case "rename":
		err = runRename(os.Args[2:])
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
