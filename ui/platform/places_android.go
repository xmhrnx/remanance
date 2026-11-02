//go:build android

package platform

import (
	"path/filepath"

	"github.com/xmhrnx/remanance/locales"
)

// storage is the phone's shared storage.
const storage = "/storage/emulated/0"

func places() []Place {
	out := []Place{{Label: locales.T("places.phone_storage"), Path: storage, Icon: "phone", Drive: true}}
	for _, d := range []struct{ name, icon string }{
		{"DCIM", "picture"}, {"Pictures", "picture"}, {"Download", "folder-open"},
		{"Documents", "document"}, {"Movies", "video"}, {"Music", "music"},
	} {
		out = append(out, Place{Label: d.name, Path: filepath.Join(storage, d.name), Icon: d.icon})
	}
	// Memory cards are mounted by their ID, such as /storage/1A2B-3C4D.
	cards, _ := filepath.Glob("/storage/*-*")
	for _, c := range cards {
		out = append(out, Place{Label: locales.T("places.memory_card"), Path: c, Icon: "sdcard", Drive: true})
	}
	return out
}
