package screens

import (
	"context"
	"os"
	"time"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/input"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/platform"
	"github.com/xmhrnx/remanance/ui/theme"
)

// FolderChooser offers the user's folders and drives as cards, the
// system's folder picker where there is one, and a field for a path.
type FolderChooser struct {
	m      *model.Model
	scope  string
	pick   func(string)
	path   *input.Input
	go_    *giode.ButtonElement
	err    string
	places []platform.Place
	listed time.Time
}

// NewFolderChooser returns a chooser that calls pick with the folder
// chosen; scope keeps its cards apart from other choosers'.
func NewFolderChooser(m *model.Model, scope, action string, pick func(string)) *FolderChooser {
	c := &FolderChooser{m: m, scope: scope, pick: pick}
	c.path = giode.Input.Text(locales.T("folderchooser.or_type_a_folder_s_path"), giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 9), BorderRadius: theme.Radius}).
		Submit(func(string) { c.typed() })
	c.go_ = components.SecondaryButton(action, "arrow-right").OnClick(c.typed)
	return c
}

func (c *FolderChooser) typed() {
	p := c.path.Text()
	if info, err := os.Stat(p); err != nil || !info.IsDir() {
		c.err = locales.T("folderchooser.there_s_no_folder_at_that")
		return
	}
	c.err = ""
	c.pick(p)
}

func (c *FolderChooser) browse() {
	go func() {
		dir, err := c.m.App.PickFolder(context.Background(), giode.FolderOptions{})
		if err == nil {
			c.m.App.Post(func() { c.pick(dir) })
		}
	}()
}

// View lays out the cards and the field.
func (c *FolderChooser) View(compact bool) giode.Element {
	// Drives come and go: list them again now and then.
	if time.Since(c.listed) > 5*time.Second {
		c.places, c.listed = platform.Places(), time.Now()
	}
	width := giode.Styles{Width: 236, Height: 72}
	if compact {
		width = giode.Styles{Width: -1, Height: 64}
	}
	var cards []giode.Element
	for _, pl := range c.places {
		pl := pl
		p := giode.Keyed(c.scope, pl.Path, func() *giode.PressableElement {
			return giode.Pressable(func(giode.PressState) giode.Element { return nil })
		})
		p.Content(func(s giode.PressState) giode.Element {
			return placeCard(pl.Icon, pl.Label, pl.Path, s)
		}).OnClick(func(giode.ClickEvent) { c.pick(pl.Path) }).Label(pl.Label)
		cards = append(cards, giode.Box(width, p))
	}
	if platform.PicksFolders {
		p := giode.Keyed(c.scope, "browse", func() *giode.PressableElement {
			return giode.Pressable(func(giode.PressState) giode.Element { return nil })
		})
		p.Content(func(s giode.PressState) giode.Element {
			return placeCard("plus", locales.T("folderchooser.another_folder"), locales.T("folderchooser.choose_any_folder"), s)
		}).OnClick(func(giode.ClickEvent) { c.browse() }).Label(locales.T("folderchooser.choose_another_folder"))
		cards = append(cards, giode.Box(width, p))
	}
	kids := []giode.Element{giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...)}
	if platform.PicksFolders {
		kids = append(kids, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{FlexGrow: 1, MaxWidth: 480}, c.path), c.go_))
	}
	if c.err != "" {
		kids = append(kids, giode.Text(c.err, theme.Caption().With(giode.Styles{Color: "theme.danger"})))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S3}, kids...)
}

// placeCard is a folder or drive to choose.
func placeCard(icon, title, sub string, s giode.PressState) giode.Element {
	st := components.CardStyles().With(giode.Styles{Width: -1, Height: -1, FlexDirection: giode.FlexDirectionRow, Wrap: giode.WrapNoWrap, Align: giode.AlignCenter,
		Gap: theme.S3, Padding: giode.SymmetricInset(theme.S4, theme.S3), BoxShadow: giode.BoxShadow{}, BorderRadius: theme.Radius})
	if s.Hovered {
		st.Background = theme.Raised()
		st.BorderColor = giode.Mix("theme.primary", "theme.border", 0.4)
	}
	return giode.Box(st,
		giode.Box(giode.Styles{Width: 36, Height: 36, BorderRadius: theme.Radius, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
			Background: giode.Mix("theme.primary", "theme.surface", 0.12)}, icons.Icon(icon, 18, "theme.primary")),
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
			giode.Text(title, theme.Body().With(giode.Styles{FontSize: 13.5, FontWeight: giode.FontWeightSemiBold, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis})),
			giode.Text(sub, theme.Caption().With(giode.Styles{MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis})),
		),
	)
}
