// Command remanence is the Remanence app: free data recovery for
// Windows, macOS, Linux, Android and iOS.
package main

import (
	"image"
	"log"
	"os"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/assets"
	_ "github.com/mehran9675/remanence/engine/fsys/all"
	"github.com/mehran9675/remanence/ui/icons"
	"github.com/mehran9675/remanence/ui/model"
	"github.com/mehran9675/remanence/ui/shell"
	"github.com/mehran9675/remanence/ui/theme"
)

func main() {
	theme.LoadFonts()
	theme.Apply()
	icons.Register()

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
