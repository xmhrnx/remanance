package main

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/iphone"
	"github.com/xmhrnx/remanance/engine/recovery"
)

func runList() error {
	infos, err := disk.List()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		fmt.Println("No drives found.")
		return nil
	}
	for _, i := range infos {
		var tags []string
		if i.Bus != "" {
			tags = append(tags, i.Bus)
		}
		if i.Removable {
			tags = append(tags, "removable")
		}
		if i.System {
			tags = append(tags, "system")
		}
		if len(i.Mounts) > 0 {
			tags = append(tags, strings.Join(i.Mounts, " "))
		}
		fmt.Printf("%-22s %9s  %s  [%s]\n", i.Path, recovery.FormatBytes(i.Size), i.Name, strings.Join(tags, ", "))
	}
	// iPhones connected by USB, for "backup".
	phones, _ := iphone.Connected()
	for _, p := range phones {
		fmt.Printf("%-22s %9s  %s  [iPhone, connected by USB: back it up first]\n", p.UDID, "", p.Name)
	}
	return nil
}
