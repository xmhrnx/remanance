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
}

// NewSpace builds the disk space screen.
func NewSpace(m *model.Model) *Space {
	s := &Space{m: m, confirm: NewConfirm()}
	s.chooser = NewFolderChooser(m, "space-place", "Measure", m.MeasureSpace)
	s.tree = components.NewTreemap(m.OpenSpace, func(n *space.Node) { m.Space.Selected = n })
	s.intro = giode.Scroll(s.introView, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	s.list = giode.VirtualList(func() int { return len(s.rows()) }, components.SizeRowHeight+2, s.row,
		giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	s.stop = components.SecondaryButton("Stop", "stop").OnClick(m.CancelSpace)
	s.back = components.IconAction("arrow-left", "Measure another folder").OnClick(m.CloseSpace)
	s.up = components.IconAction("chevron-up", "Up one folder").OnClick(func() {
		if c := m.Space.Current; c != nil && c.Parent != nil {
			m.OpenSpace(c.Parent)
		}
	})
	s.again = components.IconAction("refresh", "Measure again").OnClick(func() {
		if r := m.Space.Root; r != nil {
			m.MeasureSpace(r.Path)
		}
	})
	s.open = components.SecondaryButton("Open", "folder-open").OnClick(func() { m.OpenSpace(m.Space.Selected) })
	s.reveal = components.SecondaryButton("Show", "external").OnClick(func() {
		if n := m.Space.Selected; n != nil {
			go m.App.Share(context.Background(), giode.ShareFile{Path: n.Path})
		}
	})
	s.remove = components.SecondaryButton("Move to trash", "trash").OnClick(s.askRemove)
	if !trash.Supported {
		s.remove.Label("Delete")
	}
	return s
}

func (s *Space) askRemove() {
	n := s.m.Space.Selected
	if n == nil || n.Parent == nil {
		return
	}
	what := "file"
	if n.IsDir {
		what = "folder and the " + count(n.Files, "file", "files") + " in it"
	}
	title, msg, yes := "Move to the trash?", fmt.Sprintf("%s, the %s, goes to the trash, freeing %s. You can put it back from there.", n.Name, what, recovery.FormatBytes(n.Size)), "Move to trash"
	if !trash.Supported {
		title, msg, yes = "Delete for good?", fmt.Sprintf("%s, the %s, is deleted, freeing %s. This phone has no trash to put it back from.", n.Name, what, recovery.FormatBytes(n.Size)), "Delete"
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
		toolHeader("chart", "See what fills your drive",
			"Measure a drive or folder to see every folder's size as a map: the larger the tile, the more space it takes. Open folders to look deeper, and move what you don't need to the trash."),
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
		giode.Text("Measuring…", theme.Title()),
		giode.Text(fmt.Sprintf("%s in %s files", recovery.FormatBytes(p.Bytes), number(p.Files)), theme.Figure(18)),
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
		}).OnClick(func(giode.ClickEvent) { s.m.OpenSpace(n) }).Label("Open "+n.Name))
	}
	bar := giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		BorderWidth: 1, BorderColor: "theme.border", Background: "theme.surface"},
		s.back, s.up,
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter}, crumbs...)),
		giode.Text(fmt.Sprintf("%s · %s files", recovery.FormatBytes(cur.Size), number(int64(cur.Files))), theme.Caption()),
		s.again,
	)

	var notes []giode.Element
	if st.Err != nil {
		notes = append(notes, components.Banner("alert", "theme.danger", st.Err.Error(), nil))
	}
	if cur.Denied {
		notes = append(notes, components.Banner("lock", "theme.warning", "Some of this folder couldn't be read, so its size counts only what could.", nil))
	}

	mapView := giode.Box(giode.Styles{Width: -1, FlexGrow: 1, MinHeight: 260, Padding: giode.UniformInset(theme.S3), Background: theme.Sunken()},
		s.tree.View(cur, st.Selected))
	if len(cur.Children) == 0 {
		mapView = giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, components.EmptyState("folder", "This folder is empty", "Nothing here takes any space.", nil))
	}
	legend := s.legend(cur)
	side := giode.Stack(giode.Styles{Width: -1, Height: -1, Gap: theme.S3, Padding: giode.UniformInset(theme.S3)},
		giode.Text("Largest here", theme.Overline()),
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
	var items []giode.Element
	for i, k := range n.Kinds() {
		if i == 6 || k.Size == 0 {
			break
		}
		items = append(items, components.KindSwatch(k.Kind, k.Kind.Label()+" "+recovery.FormatBytes(k.Size)))
	}
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S4, Align: giode.AlignCenter}, items...)
}

// selection is the selected item's details and actions.
func (s *Space) selection() giode.Element {
	n := s.m.Space.Selected
	if n == nil {
		return giode.Text("Click a tile or a row to select it; click it again to open a folder.", theme.Caption())
	}
	kind := space.KindOf(n.Name).Label()
	if n.IsDir {
		kind = fmt.Sprintf("Folder · %s files", number(int64(n.Files)))
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
		return fmt.Sprintf("%.1f million", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprint(n)
}
