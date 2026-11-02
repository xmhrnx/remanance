package screens

import (
	"context"
	"fmt"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/space"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/platform"
	"github.com/xmhrnx/remanance/ui/theme"
)

// maxListed is how many of a folder's items the list beside the map
// shows.
const maxListed = 200

// Space shows what fills a drive or folder: a treemap of it, and its
// largest files and folders in a list beside it.
type Space struct {
	m       *model.Model
	chooser *FolderChooser
	tree    *components.Treemap
	confirm *Confirm
	intro   *scroll.Scroll
	list    *giode.VirtualListElement
	stop    *giode.ButtonElement
	back    *iconbutton.Button
	up      *iconbutton.Button
	again   *iconbutton.Button
	open    *giode.ButtonElement
	reveal  *giode.ButtonElement
	remove  *giode.ButtonElement
	kinds   kindsOf
}

// kindsOf keeps a folder's totals by kind: summing them walks every
// file below it, too slow to do each frame on a whole drive. Size and
// files tell when something in it was deleted.
type kindsOf struct {
	n     *space.Node
	size  int64
	files int
	kinds []space.KindTotal
}

// NewSpace builds the disk space screen.
func NewSpace(m *model.Model) *Space {
	s := &Space{m: m, confirm: NewConfirm()}
	s.chooser = NewFolderChooser(m, "space-place", locales.T("space.measure"), m.MeasureSpace)
	s.tree = components.NewTreemap(m.OpenSpace, func(n *space.Node) { m.Space.Selected = n })
	s.intro = giode.Scroll(s.introView, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	s.list = giode.VirtualList(func() int { return len(s.rows()) }, components.SizeRowHeight+2, s.row,
		giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	s.stop = components.SecondaryButton(locales.T("space.stop"), "stop").OnClick(m.CancelSpace)
	s.back = components.IconAction("arrow-left", locales.T("space.measure_another_folder")).OnClick(m.CloseSpace)
	s.up = components.IconAction("chevron-up", locales.T("space.up_one_folder")).OnClick(func() {
		if c := m.Space.Current; c != nil && c.Parent != nil {
			m.OpenSpace(c.Parent)
		}
	})
	s.again = components.IconAction("refresh", locales.T("space.measure_again")).OnClick(func() {
		if r := m.Space.Root; r != nil {
			m.MeasureSpace(r.Path)
		}
	})
	s.open = components.SecondaryButton(locales.T("space.open"), "folder-open").OnClick(func() { m.OpenSpace(m.Space.Selected) })
	s.reveal = components.SecondaryButton(locales.T("space.show"), "external").OnClick(func() {
		if n := m.Space.Selected; n != nil {
			go m.App.Share(context.Background(), giode.ShareFile{Path: n.Path})
		}
	})
	s.remove = components.SecondaryButton(locales.T("space.move_to_trash"), "trash").OnClick(s.askRemove)
	if !trash.Supported {
		s.remove.Label(locales.T("space.delete"))
	}
	return s
}

func (s *Space) askRemove() {
	n := s.m.Space.Selected
	if n == nil || n.Parent == nil {
		return
	}
	what := locales.T("space.the_file")
	if n.IsDir {
		what = locales.T("space.the_folder", "files", count(n.Files, "file"))
	}
	size := recovery.FormatBytes(n.Size)
	title, msg, yes := locales.T("space.move_to_the_trash"), locales.T("space.trash_message", "name", n.Name, "what", what, "size", size), locales.T("space.move_to_trash")
	if !trash.Supported {
		title, msg, yes = locales.T("space.delete_for_good"), locales.T("space.delete_message", "name", n.Name, "what", what, "size", size), locales.T("space.delete")
	}
	s.confirm.Ask(title, msg, yes, func() { s.m.TrashSpace(n) })
}

func (s *Space) rows() []*space.Node {
	c := s.m.Space.Current
	if c == nil {
		return nil
	}
	if len(c.Children) > maxListed {
		return c.Children[:maxListed]
	}
	return c.Children
}

func (s *Space) row(i int) giode.Element {
	rows := s.rows()
	if i >= len(rows) {
		return giode.Box(giode.Styles{})
	}
	n := rows[i]
	of := s.m.Space.Current.Size
	p := giode.Keyed("space-row", n, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	return giode.Box(giode.Styles{Width: -1, Padding: giode.Inset{Bottom: 2}}, p.Content(func(st giode.PressState) giode.Element {
		return components.SizeRow(n, i, of, n == s.m.Space.Selected, st)
	}).OnClick(func(e giode.ClickEvent) {
		if n.IsDir && (e.Clicks == 2 || n == s.m.Space.Selected) {
			giode.AfterLayout(func() { s.m.OpenSpace(n) })
			return
		}
		s.m.Space.Selected = n
	}).Label(n.Name))
}

// View lays out the screen.
func (s *Space) View() giode.Element {
	st := &s.m.Space
	var body giode.Element
	switch {
	case st.Measuring:
		body = s.measuring()
	case st.Root != nil:
		body = s.results()
	default:
		body = s.intro
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1}, giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, body), s.confirm.View())
}

func (s *Space) introView() giode.Element {
	compact := giode.Compact()
	kids := []giode.Element{
		toolHeader("chart", locales.T("space.see_what_fills_your_drive"),
			locales.T("space.measure_a_drive_or_folder_to")),
	}
	if err := s.m.Space.Err; err != nil {
		kids = append(kids, components.Banner("alert", "theme.danger", err.Error(), nil))
	}
	kids = append(kids, s.chooser.View(compact))
	return page(compact, kids...)
}

func (s *Space) measuring() giode.Element {
	p := s.m.Space.Progress
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: theme.S4, Padding: giode.UniformInset(theme.S6)},
		giode.Spinner(giode.Styles{Width: 36, Height: 36, Color: "theme.primary"}),
		giode.Text(locales.T("space.measuring"), theme.Title()),
		giode.Text(locales.T("space.measured", "size", recovery.FormatBytes(p.Bytes), "files", number(p.Files)), theme.Figure(18)),
		giode.Box(giode.Styles{MaxWidth: 560}, giode.Text(p.Path, theme.Caption().With(giode.Styles{TextAlign: giode.TextAlignCenter, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis}))),
		s.stop,
	)
}

func (s *Space) results() giode.Element {
	st := &s.m.Space
	compact := giode.Compact()
	cur := st.Current

	var crumbs []giode.Element
	trail := cur.Trail()
	for i, n := range trail {
		n := n
		if i > 0 {
			crumbs = append(crumbs, icons.Icon("chevron-right", 14, "theme.muted"))
		}
		style := theme.Body().With(giode.Styles{FontSize: 13, MaxLines: 1})
		if n == cur {
			style.FontWeight = giode.FontWeightSemiBold
			crumbs = append(crumbs, giode.Text(n.Name, style))
			continue
		}
		style.Color = "theme.muted"
		b := giode.Keyed("space-crumb", n, func() *giode.PressableElement {
			return giode.Pressable(func(giode.PressState) giode.Element { return nil })
		})
		crumbs = append(crumbs, b.Content(func(ps giode.PressState) giode.Element {
			if ps.Hovered {
				return giode.Text(n.Name, style.With(giode.Styles{Color: "theme.text"}))
			}
			return giode.Text(n.Name, style)
		}).OnClick(func(giode.ClickEvent) { s.m.OpenSpace(n) }).Label(locales.T("space.open_name", "name", n.Name)))
	}
	bar := giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		BorderWidth: 1, BorderColor: "theme.border", Background: "theme.surface"},
		s.back, s.up,
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter}, crumbs...)),
		giode.Text(locales.T("space.size_files", "size", recovery.FormatBytes(cur.Size), "files", number(int64(cur.Files))), theme.Caption()),
		s.again,
	)

	var notes []giode.Element
	if st.Err != nil {
		notes = append(notes, components.Banner("alert", "theme.danger", st.Err.Error(), nil))
	}
	if cur.Denied {
		notes = append(notes, components.Banner("lock", "theme.warning", locales.T("space.some_of_this_folder_couldn_t"), nil))
	}

	mapView := giode.Box(giode.Styles{Width: -1, FlexGrow: 1, MinHeight: 260, Padding: giode.UniformInset(theme.S3), Background: theme.Sunken()},
		s.tree.View(cur, st.Selected))
	if len(cur.Children) == 0 {
		mapView = giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, components.EmptyState("folder", locales.T("space.this_folder_is_empty"), locales.T("space.nothing_here_takes_any_space"), nil))
	}
	legend := s.legend(cur)
	side := giode.Stack(giode.Styles{Width: -1, Height: -1, Gap: theme.S3, Padding: giode.UniformInset(theme.S3)},
		giode.Text(locales.T("space.largest_here"), theme.Overline()),
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, s.list),
		s.selection(),
	)
	if compact {
		return giode.Stack(giode.Styles{Width: -1, Height: -1}, append(append([]giode.Element{bar}, notes...),
			giode.Box(giode.Styles{Width: -1, Height: 300, Padding: giode.UniformInset(theme.S2), Background: theme.Sunken()}, s.tree.View(cur, st.Selected)),
			giode.Box(giode.Styles{Width: -1, Padding: giode.SymmetricInset(theme.S3, theme.S2)}, legend),
			giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, side))...)
	}
	main := giode.Stack(giode.Styles{FlexGrow: 1, Height: -1},
		mapView,
		giode.Box(giode.Styles{Width: -1, Padding: giode.SymmetricInset(theme.S4, theme.S3), BorderWidth: 1, BorderColor: "theme.border"}, legend),
	)
	return giode.Stack(giode.Styles{Width: -1, Height: -1}, append(append([]giode.Element{bar}, notes...),
		giode.Row(giode.Styles{Width: -1, FlexGrow: 1, Wrap: giode.WrapNoWrap},
			main,
			giode.Box(giode.Styles{Width: 340, Height: -1, Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, side),
		))...)
}

// legend sums the folder's files by kind, largest first.
func (s *Space) legend(n *space.Node) giode.Element {
	c := &s.kinds
	if c.n != n || c.size != n.Size || c.files != n.Files {
		*c = kindsOf{n: n, size: n.Size, files: n.Files, kinds: n.Kinds()}
	}
	var items []giode.Element
	for i, k := range c.kinds {
		if i == 6 || k.Size == 0 {
			break
		}
		items = append(items, components.KindSwatch(k.Kind, locales.T("space.kind_size", "kind", label(k.Kind.Label()), "size", recovery.FormatBytes(k.Size))))
	}
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S4, Align: giode.AlignCenter}, items...)
}

// selection is the selected item's details and actions.
func (s *Space) selection() giode.Element {
	n := s.m.Space.Selected
	if n == nil {
		return giode.Text(locales.T("space.click_a_tile_or_a_row"), theme.Caption())
	}
	kind := label(space.KindOf(n.Name).Label())
	if n.IsDir {
		kind = locales.T("space.folder_files", "files", number(int64(n.Files)))
	}
	actions := []giode.Element{}
	if n.IsDir {
		actions = append(actions, s.open)
	}
	if platform.CanReveal {
		actions = append(actions, s.reveal)
	}
	actions = append(actions, s.remove)
	return components.Card(giode.Styles{Width: -1, Padding: giode.UniformInset(theme.S4), Gap: theme.S2, BoxShadow: giode.BoxShadow{}},
		giode.Text(n.Name, theme.Heading().With(giode.Styles{FontSize: 15, MaxLines: 2})),
		giode.Text(recovery.FormatBytes(n.Size)+" · "+kind, theme.Caption()),
		giode.Text(n.Path, theme.Caption().With(giode.Styles{FontSize: 11, MaxLines: 2, TextOverflow: giode.TextOverflowEllipsis})),
		giode.Row(giode.Styles{Gap: theme.S2, Padding: giode.Inset{Top: theme.S1}}, actions...),
	)
}

// number writes a large count readably.
func number(n int64) string {
	switch {
	case n >= 1_000_000:
		return locales.T("space.million", "n", fmt.Sprintf("%.1f", float64(n)/1e6))
	case n >= 10_000:
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprint(n)
}
