package screens

import (
	"context"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/space"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/model"
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
