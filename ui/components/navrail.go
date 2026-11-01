package components

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// NavRailWidth is how wide the tool rail is on a desktop.
const NavRailWidth = 84

// NavRailItem is one tool in the rail along the window's left edge: its
// icon in a pill that lights up when it's the tool on show, and its name
// beneath.
func NavRailItem(t model.ToolInfo, active bool, s giode.PressState) giode.Element {
	pill := giode.Styles{Width: 52, Height: 32, BorderRadius: 16, Align: giode.AlignCenter, Justify: giode.JustifyCenter}
	color := "theme.muted"
	label := theme.Caption().With(giode.Styles{FontSize: 11.5, TextAlign: giode.TextAlignCenter})
	switch {
	case active:
		pill.Background = giode.Mix("theme.primary", "theme.surface", 0.18)
		color = "theme.primary"
		label.Color, label.FontWeight = "theme.text", giode.FontWeightSemiBold
	case s.Hovered:
		pill.Background = theme.Raised()
		color = "theme.text"
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 4, Align: giode.AlignCenter, Padding: giode.SymmetricInset(0, 6)},
		giode.Box(pill, icons.Icon(t.Icon, 20, color)),
		giode.Text(t.Label, label),
	)
}

// TabBarItem is one tool in the tab bar along a phone's bottom edge.
func TabBarItem(t model.ToolInfo, active bool, s giode.PressState) giode.Element {
	color := "theme.muted"
	pill := giode.Styles{Width: 48, Height: 28, BorderRadius: 14, Align: giode.AlignCenter, Justify: giode.JustifyCenter}
	label := theme.Caption().With(giode.Styles{FontSize: 10.5, TextAlign: giode.TextAlignCenter, MaxLines: 1})
	if active {
		color = "theme.primary"
		pill.Background = giode.Mix("theme.primary", "theme.surface", 0.18)
		label.Color, label.FontWeight = "theme.text", giode.FontWeightSemiBold
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 3, Align: giode.AlignCenter, Padding: giode.SymmetricInset(0, 6)},
		giode.Box(pill, icons.Icon(t.Icon, 19, color)),
		giode.Text(t.Short, label),
	)
}
