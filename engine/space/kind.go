package space

import (
	"path"
	"strings"
	"time"
)

// Kind is what a file is, for coloring the treemap.
type Kind int

const (
	KindOther Kind = iota
	KindPicture
	KindVideo
	KindAudio
	KindDocument
	KindArchive
	KindCode
	KindApp
	KindSystem
)

// Label names a kind.
func (k Kind) Label() string {
	return [...]string{"Other", "Pictures", "Videos", "Music and audio", "Documents", "Archives and disk images",
		"Code", "Apps and programs", "System files"}[k]
}

var kinds = map[string]Kind{}

func init() {
	for k, exts := range map[Kind]string{
		KindPicture:  "jpg jpeg png gif webp heic heif bmp tif tiff dng raw cr2 cr3 nef arw orf rw2 psd svg ico avif",
		KindVideo:    "mp4 m4v mov avi mkv webm wmv flv 3gp mts m2ts mpg mpeg vob",
		KindAudio:    "mp3 m4a aac flac wav ogg opus wma aiff alac amr mid",
		KindDocument: "pdf doc docx xls xlsx ppt pptx odt ods odp rtf txt md csv epub pages numbers key",
		KindArchive:  "zip rar 7z tar gz tgz bz2 xz zst iso img dmg vhd vhdx vmdk qcow2 cab",
		KindCode:     "go c h cpp hpp cs java kt js ts tsx jsx py rb rs swift php html css json xml yaml yml toml sql sh",
		KindApp:      "exe msi dll app apk aab ipa deb rpm appimage msix jar so dylib",
		KindSystem:   "sys bin dat db sqlite log tmp cache pak idx lock",
	} {
		for _, e := range strings.Fields(exts) {
			kinds[e] = k
		}
	}
}

// KindOf tells what a file is from its name.
func KindOf(name string) Kind {
	return kinds[strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")]
}

func newTicker() *time.Ticker { return time.NewTicker(150 * time.Millisecond) }
