package screens

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/cleanup"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

func (c *Cleanup) content() giode.Element {
	st := &c.m.Clean
	compact := giode.Compact()
	kids := []giode.Element{
		toolHeader("broom", locales.T("cleanup.free_up_space"), locales.T("cleanup.intro")),
	}
	if st.Space.Total > 0 {
		kids = append(kids, c.drive())
	}
	if st.Done != "" {
		kids = append(kids, components.Banner("check", "theme.success", st.Done, nil))
	}
	for _, err := range []error{st.Failed, st.Err} {
		if err != nil {
			kids = append(kids, components.Banner("alert", "theme.danger", err.Error(), nil))
		}
	}
	switch {
	case st.Looking:
		kids = append(kids, c.looking())
	case len(st.Groups) == 0 && st.Err == nil:
		kids = append(kids, components.EmptyState("check", locales.T("cleanup.nothing_to_clean"), locales.T("cleanup.nothing_here_can_go"), nil))
	case len(st.Groups) > 0:
		var cards []giode.Element
		for _, g := range st.Groups {
			cards = append(cards, c.group(g))
		}
		kids = append(kids, section(locales.T("cleanup.what_can_go"), cards...))
	}
	kids = append(kids, section(locales.T("cleanup.more_ways"), c.more(compact)))
	return page(compact, kids...)
}

// drive shows how full the drive is.
func (c *Cleanup) drive() giode.Element {
	s := c.m.Clean.Space
	used := float32(s.Total-s.Free) / float32(s.Total)
	color := "theme.primary"
	if s.Free < s.Total/10 {
		color = "theme.danger"
	}
	return components.Card(giode.Styles{Width: -1, Gap: theme.S3},
		giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			icons.Icon("drive", 20, color),
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(locales.T("cleanup.free_of", "free", recovery.FormatBytes(s.Free), "total", recovery.FormatBytes(s.Total)), theme.Heading())),
			giode.Text(cleanup.Drive(), theme.Caption().With(giode.Styles{MaxLines: 1})),
		),
		giode.Progress(used, giode.Styles{Height: 8, BorderRadius: 4, Color: color, Background: theme.Sunken()}),
	)
}

func (c *Cleanup) looking() giode.Element {
	p := c.m.Clean.Progress
	detail := locales.T("cleanup.found_so_far", "files", count(p.Files, "file"), "size", recovery.FormatBytes(p.Bytes))
	return components.Card(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Padding: giode.UniformInset(theme.S6)},
		giode.Spinner(giode.Styles{Width: 32, Height: 32, Color: "theme.primary"}),
		giode.Text(locales.T("cleanup.looking_in", "what", label(p.Kind.Label())), theme.Heading()),
		giode.Text(detail, theme.Secondary()),
		c.stop,
	)
}

// more leads to the tools that find big and repeated files, which take
// a person's judgment to clean.
func (c *Cleanup) more(compact bool) giode.Element {
	size := giode.Styles{Width: 300, Height: 72}
	if compact {
		size = giode.Styles{Width: -1, Height: 64}
	}
	var cards []giode.Element
	for _, t := range []struct {
		tool       model.Tool
		icon, sub  string
		title, key string
	}{
		{model.ToolSpace, "chart", locales.T("cleanup.see_what_takes_the_most_room"), locales.T("tool.disk_space"), "space"},
		{model.ToolDupes, "copy", locales.T("cleanup.find_copies_of_the_same_file"), locales.T("tool.duplicates"), "dupes"},
	} {
		t := t
		p := giode.Keyed("clean-more", t.key, func() *giode.PressableElement {
			return giode.Pressable(func(giode.PressState) giode.Element { return nil })
		})
		p.Content(func(s giode.PressState) giode.Element {
			return placeCard(t.icon, t.title, t.sub, s)
		}).OnClick(func(giode.ClickEvent) { c.m.Tool = t.tool }).Label(t.title)
		cards = append(cards, giode.Box(size, p))
	}
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...)
}

// foot is the bar along the bottom: what's checked, and the button that
// cleans it.
func (c *Cleanup) foot() giode.Element {
	st := &c.m.Clean
	items, size := c.m.CleanChecked()
	c.clean.Disabled(len(items) == 0 || st.Cleaning)
	c.clean.Label(locales.T("cleanup.clean_size", "size", recovery.FormatBytes(size)))
	if st.Cleaning {
		c.clean.Label(locales.T("cleanup.cleaning_progress", "done", st.Cleaned[0], "of", st.Cleaned[1]))
	}
	c.again.Disabled(st.Cleaning)
	summary := locales.T("cleanup.nothing_checked")
	if len(items) > 0 {
		summary = locales.T("cleanup.checked", "size", recovery.FormatBytes(size))
	}
	kids := []giode.Element{giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(summary, theme.Secondary().With(giode.Styles{MaxLines: 2})))}
	if !giode.Compact() {
		kids = append(kids, c.again)
	}
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, append(kids, c.clean)...)
}
