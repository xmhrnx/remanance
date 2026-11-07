package screens

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/cleanup"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

// group is a kind of file found: a box to check it all, and its items
// below when opened.
func (c *Cleanup) group(g cleanup.Group) giode.Element {
	checked := c.m.CleanGroupChecked(g)
	box := giode.Keyed("clean-check", g.Kind, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	box.Content(func(s giode.PressState) giode.Element {
		return giode.Box(giode.Styles{Padding: giode.UniformInset(theme.S2)}, components.CheckMark(checked > 0, s.Hovered))
	}).OnClick(func(giode.ClickEvent) { c.m.ToggleCleanGroup(g) }).Label(label(g.Kind.Label()))

	head := giode.Keyed("clean-head", g.Kind, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	open := c.open[g.Kind]
	head.Content(func(s giode.PressState) giode.Element {
		chevron := "chevron-down"
		if open {
			chevron = "chevron-up"
		}
		status := count(g.Files(), "file")
		if checked > 0 && checked < len(g.Items) {
			status = locales.T("cleanup.checked_of", "n", checked, "of", len(g.Items))
		}
		title := theme.Body().With(giode.Styles{FontSize: 14, FontWeight: giode.FontWeightSemiBold})
		if s.Hovered {
			title.Color = "theme.primary"
		}
		return giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 36, Height: 36, BorderRadius: theme.Radius, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
				Background: giode.Mix("theme.primary", "theme.surface", 0.12)}, icons.Icon(cleanKinds[g.Kind].icon, 18, "theme.primary")),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 3},
				giode.Text(label(g.Kind.Label()), title),
				giode.Text(cleanKinds[g.Kind].about, theme.Caption())),
			giode.Stack(giode.Styles{Gap: 3, Align: giode.AlignEnd},
				giode.Text(recovery.FormatBytes(g.Size()), theme.Body().With(giode.Styles{FontSize: 14, FontWeight: giode.FontWeightSemiBold})),
				giode.Text(status, theme.Caption().With(giode.Styles{MaxLines: 1}))),
			icons.Icon(chevron, 16, "theme.muted"),
		)
	}).OnClick(func(giode.ClickEvent) { c.open[g.Kind] = !open }).Label(locales.T("cleanup.show_what_s_in", "what", label(g.Kind.Label())))

	kids := []giode.Element{giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		box, giode.Box(giode.Styles{FlexGrow: 1}, head))}
	if open {
		for i, it := range g.Items {
			if i == maxCleanRows {
				kids = append(kids, giode.Text(locales.T("cleanup.and_more", "n", len(g.Items)-i), theme.Caption().With(giode.Styles{Padding: giode.Inset{Left: theme.S2}})))
				break
			}
			kids = append(kids, c.item(it))
		}
	}
	return components.Card(giode.Styles{Width: -1, Gap: theme.S2, Padding: giode.SymmetricInset(theme.S3, theme.S3), BoxShadow: giode.BoxShadow{}}, kids...)
}

// item is one file or folder of a group, to check or uncheck.
func (c *Cleanup) item(it cleanup.Item) giode.Element {
	p := giode.Keyed("clean-item", it.Path, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	on := c.m.Clean.Checked[it.Path]
	return p.Content(func(s giode.PressState) giode.Element {
		bg := "transparent"
		if s.Hovered {
			bg = theme.Raised()
		}
		facts := []string{recovery.FormatBytes(it.Size)}
		if it.Kind == cleanup.Installers || it.Kind == cleanup.OldDownloads {
			facts = append(facts, it.Modified.Format("Jan 2, 2006"))
		}
		where := it.Path
		if it.Kind == cleanup.Trash {
			where = count(it.Files, "file")
		}
		return giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
			Padding: giode.SymmetricInset(theme.S2, 6), Background: bg, BorderRadius: theme.RadiusSmall},
			components.CheckMark(on, s.Hovered),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
				giode.Text(label(it.Title()), theme.Body().With(giode.Styles{FontSize: 13, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis})),
				giode.Text(where, theme.Caption().With(giode.Styles{FontSize: 11.5, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis}))),
			giode.Text(join(facts), theme.Caption().With(giode.Styles{MaxLines: 1})),
		)
	}).OnClick(func(giode.ClickEvent) { c.m.ToggleCleanItem(it.Path) }).Label(it.Title())
}
