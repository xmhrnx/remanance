// Package shell is the app's frame: the title bar, the page on show and
// the toasts above everything.
package shell

import (
	"runtime"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/titlebar"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/screens"
	"github.com/xmhrnx/remanance/ui/theme"
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
	space   *screens.Space
	dupes   *screens.Dupes
	sync    *screens.Sync
	rename  *screens.Rename
}

// New builds the frame around m.
func New(app *giode.App, m *model.Model) *Shell {
	s := &Shell{m: m}
	s.drives = screens.NewDrives(m)
	s.scan = screens.NewScanning(m)
	s.results = screens.NewResults(m)
	s.space = screens.NewSpace(m)
	s.dupes = screens.NewDupes(m)
	s.sync = screens.NewSync(m)
	s.rename = screens.NewRename(m)
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
		giode.Breakpoint{MaxWidth: 760, Name: "compact", View: s.compact},
		giode.Breakpoint{View: s.wide},
	)
	kids := []giode.Element{}
	if Desktop && !s.m.App.Fullscreen() {
		kids = append(kids, s.bar)
	}
	kids = append(kids, giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, page))
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: "theme.background"}, kids...)
}

// wide puts the tools in a rail along the left edge.
func (s *Shell) wide() giode.Element {
	items := []giode.Element{}
	for _, t := range model.Tools {
		items = append(items, s.navItem(t, components.NavRailItem))
	}
	rail := giode.Stack(giode.Styles{Width: components.NavRailWidth, Height: -1, Gap: theme.S1, Padding: giode.SymmetricInset(theme.S2, theme.S4),
		Background: "theme.background", BorderWidth: 1, BorderColor: "theme.border"}, items...)
	return giode.Row(giode.Styles{Width: -1, Height: -1, Wrap: giode.WrapNoWrap},
		rail, giode.Box(giode.Styles{FlexGrow: 1, Height: -1}, s.page()))
}

// compact puts the tools in a tab bar along a phone's bottom edge.
func (s *Shell) compact() giode.Element {
	items := []giode.Element{}
	for _, t := range model.Tools {
		items = append(items, giode.Box(giode.Styles{FlexGrow: 1}, s.navItem(t, components.TabBarItem)))
	}
	bar := giode.Row(giode.Styles{Width: -1, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S1, theme.S1),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, items...)
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, s.page()), bar)
}

// navItem is a tool's button in the rail or tab bar.
func (s *Shell) navItem(t model.ToolInfo, draw func(model.ToolInfo, bool, giode.PressState) giode.Element) giode.Element {
	active := s.m.Tool == t.Tool
	p := giode.Keyed("tool-nav", t.Label, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	return p.Content(func(st giode.PressState) giode.Element { return draw(t, active, st) }).
		OnClick(func(giode.ClickEvent) { s.m.Tool = t.Tool }).Label(t.Label)
}

func (s *Shell) page() giode.Element {
	switch s.m.Tool {
	case model.ToolSpace:
		return s.space.View()
	case model.ToolDupes:
		return s.dupes.View()
	case model.ToolSync:
		return s.sync.View()
	case model.ToolRename:
		return s.rename.View()
	}
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
