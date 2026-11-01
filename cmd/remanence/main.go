// Command remanence is the Remanence app: free data recovery for
// Windows, macOS, Linux, Android and iOS.
package main

import (
	"image"
	"log"
	"os"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/assets"
	_ "github.com/xmhrnx/remanance/engine/fsys/all"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/platform/adbphones"
	"github.com/xmhrnx/remanance/ui/platform/androidroot"
	"github.com/xmhrnx/remanance/ui/platform/androidstore"
	"github.com/xmhrnx/remanance/ui/platform/androidusb"
	"github.com/xmhrnx/remanance/ui/platform/iosbackups"
	"github.com/xmhrnx/remanance/ui/platform/iphones"
	"github.com/xmhrnx/remanance/ui/shell"
	"github.com/xmhrnx/remanance/ui/theme"
)

func main() {
	theme.LoadFonts()
	theme.Apply()
	icons.Register()
	androidstore.Register()
	androidusb.Register()
	androidroot.Register()
	iosbackups.Register()
	adbphones.Register()
	iphones.Register()

	var m *model.Model
	app := giode.New(giode.Config{
		Title:      "Remanence",
		Size:       image.Pt(1280, 820),
		MinSize:    image.Pt(420, 560),
		Resizable:  true,
		Frameless:  shell.Desktop,
		Background: "theme.background",
		Icon:       assets.Icon,
		OnShutdown: func() {
			if m != nil {
				m.Close()
			}
		},
	})
	m = model.New(app)
	s := shell.New(app, m)
	m.LoadDrives()
	// Folders synced with other devices keep syncing from launch.
	if model.SyncConfigured() {
		m.StartSync()
	}
	// A disk image opened with the app.
	if len(os.Args) > 1 {
		if err := s.OpenImage(os.Args[1]); err != nil {
			log.Print(err)
		}
	}
	if err := app.Run(s.View); err != nil {
		log.Fatal(err)
	}
}
