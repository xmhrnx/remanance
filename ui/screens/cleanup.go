package screens

import (
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/xmhrnx/remanance/engine/cleanup"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// maxCleanRows is how many of a group's items are listed when it's
// opened; the rest are checked and unchecked with the group.
const maxCleanRows = 50

// cleanKinds say what each kind of file is, and its icon.
var cleanKinds = map[cleanup.Kind]struct{ icon, about string }{
	cleanup.Temp:         {"clock", locales.T("cleanup.about_temp")},
	cleanup.Caches:       {"layers", locales.T("cleanup.about_caches")},
	cleanup.BuildCaches:  {"box", locales.T("cleanup.about_build_caches")},
	cleanup.Logs:         {"document", locales.T("cleanup.about_logs")},
	cleanup.Trash:        {"trash", locales.T("cleanup.about_trash")},
	cleanup.Installers:   {"archive", locales.T("cleanup.about_installers")},
	cleanup.OldDownloads: {"history", locales.T("cleanup.about_old_downloads")},
}

// Cleanup frees space: it finds temporary files, caches, logs, the
// trash and forgotten downloads, and cleans the ones checked.
type Cleanup struct {
	m       *model.Model
	scroll  *scroll.Scroll
	open    map[cleanup.Kind]bool // groups showing their items
	again   *giode.ButtonElement
	stop    *giode.ButtonElement
	clean   *giode.ButtonElement
	confirm *Confirm
}

// NewCleanup builds the clean up screen.
func NewCleanup(m *model.Model) *Cleanup {
	c := &Cleanup{m: m, open: map[cleanup.Kind]bool{}, confirm: NewConfirm()}
	c.scroll = giode.Scroll(c.content, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	c.again = components.GhostButton(locales.T("cleanup.look_again"), "refresh").OnClick(m.FindCleanup)
	c.stop = components.SecondaryButton(locales.T("cleanup.stop"), "stop").OnClick(m.CancelCleanup)
	c.clean = components.PrimaryButton("", "broom").OnClick(c.askClean)
	return c
}

// askClean says what cleaning the checked items will do, and does it if
// the user agrees.
func (c *Cleanup) askClean() {
	items, size := c.m.CleanChecked()
	if len(items) == 0 {
		return
	}
	var deletes, empties, trashes bool
	for _, it := range items {
		switch {
		case it.Kind == cleanup.Trash:
			empties = true
		case it.Kind.ToTrash():
			trashes = true
		default:
			deletes = true
		}
	}
	var msg []string
	if deletes {
		msg = append(msg, locales.T("cleanup.confirm_delete"))
	}
	if empties {
		msg = append(msg, locales.T("cleanup.confirm_empty"))
	}
	if trashes && trash.Supported {
		msg = append(msg, locales.T("cleanup.confirm_trash"))
	} else if trashes {
		msg = append(msg, locales.T("cleanup.confirm_downloads_deleted"))
	}
	c.confirm.Ask(locales.T("cleanup.confirm_title", "size", recovery.FormatBytes(size)), strings.Join(msg, " "), locales.T("cleanup.clean_up"), c.m.CleanUp)
}

// View lays out the screen, looking the first time it's shown.
func (c *Cleanup) View() giode.Element {
	st := &c.m.Clean
	if !st.Looked {
		c.m.FindCleanup()
	}
	kids := []giode.Element{giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, c.scroll)}
	if !st.Looking && len(st.Groups) > 0 {
		kids = append(kids, c.foot())
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1}, append(kids, c.confirm.View())...)
}

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
