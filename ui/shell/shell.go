// Package shell is the app's frame: the title bar, the page on show and
// the toasts above everything.
package shell

import (
	"runtime"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/titlebar"
	"github.com/mehran9675/remanence/ui/components"
	"github.com/mehran9675/remanence/ui/icons"
	"github.com/mehran9675/remanence/ui/model"
	"github.com/mehran9675/remanence/ui/screens"
	"github.com/mehran9675/remanence/ui/theme"
)

// Desktop is true where the window draws its own title bar.
const Desktop = runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "linux"

// Shell lays out the whole window.
type Shell struct {
	m       *model.Model
	bar     *titlebar.TitleBar
	mode    *iconbutton.Button
	drives  *screens.Drives
	scan    *screens.Scanning
	results *screens.Results
}

// New builds the frame around m.
func New(app *giode.App, m *model.Model) *Shell {
	s := &Shell{m: m}
	s.drives = screens.NewDrives(m)
	s.scan = screens.NewScanning(m)
	s.results = screens.NewResults(m)
	s.mode = components.IconAction("sun", "Switch between light and dark").OnClick(func() {
		giode.ToggleMode()
		giode.InvalidateCaches()
	})
	variant := giode.TitleBarWindows
	if runtime.GOOS == "darwin" {
		variant = giode.TitleBarMac
	}
	s.bar = giode.TitleBar(app, "", giode.Styles{Background: "theme.background", Height: 44, BorderWidth: 1, BorderColor: "theme.border"}).
		Icon(giode.Box(giode.Styles{Padding: giode.Inset{Left: theme.S2}}, components.Brand())).
		Trailing(s.mode).
		Variant(variant)
	return s
}

// View lays out the frame and the page on show.
func (s *Shell) View() giode.Element {
	if giode.CurrentMode() == giode.ModeDark {
		s.mode.Icon(icons.Icon("sun", 18, ""))
	} else {
		s.mode.Icon(icons.Icon("moon", 18, ""))
	}
	// The page is built inside the breakpoint, so the screens can ask
	// giode.Compact() while they build.
	page := giode.Responsive(
		giode.Breakpoint{MaxWidth: 760, Name: "compact", View: s.page},
		giode.Breakpoint{View: s.page},
	)
	kids := []giode.Element{}
	if Desktop && !s.m.App.Fullscreen() {
		kids = append(kids, s.bar)
	}
	kids = append(kids, giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, page))
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: "theme.background"}, kids...)
}

func (s *Shell) page() giode.Element {
	switch s.m.Screen {
	case model.ScreenScan:
		return s.scan.View()
	case model.ScreenResults:
		return s.results.View()
	}
	return s.drives.View()
}

// OpenImage lists a disk image given on the command line.
func (s *Shell) OpenImage(path string) error { return s.drives.AddImage(path) }
