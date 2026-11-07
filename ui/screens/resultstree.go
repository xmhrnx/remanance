package screens

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// syncTree follows the model: new results replace the tree's roots, and
// a folder opened from the table is selected and revealed in the tree.
func (r *Results) syncTree() {
	if len(r.treeFrom) != len(r.m.Roots) || (len(r.treeFrom) > 0 && r.treeFrom[0] != r.m.Roots[0]) {
		r.treeFrom = r.m.Roots
		r.treeRoots = r.treeRoots[:0]
		for _, n := range r.m.Roots {
			r.treeRoots = append(r.treeRoots, n)
		}
		if len(r.m.Roots) > 0 {
			r.tree.Expand(r.m.Roots[0], true)
		}
		r.shownDir = nil
	}
	if f := r.m.Folder; f != nil && f != r.shownDir {
		r.shownDir = f
		for _, a := range f.Trail()[:len(f.Trail())-1] {
			r.tree.Expand(a, true)
		}
		r.tree.Select(f)
		r.tree.Reveal(f)
	}
}

func (r *Results) treeRow(tn giode.TreeNode, s giode.TreeRowState) giode.Element {
	n := tn.(*model.Node)
	icon, color := "folder", giode.Mix("theme.text", "theme.surface", 0.55)
	if n.Entry.State != fsys.Existing {
		color = "theme.primary"
	}
	title := theme.Body().With(giode.Styles{FontSize: 13, TextOverflow: giode.TextOverflowEllipsis})
	if s.Selected {
		title.Color = "theme.text"
		title.FontWeight = giode.FontWeightMedium
	}
	var extra []giode.Element
	switch n.Kind {
	case model.KindVolume:
		icon, color = "drive", "theme.text"
		title.FontWeight = giode.FontWeightSemiBold
		if n.Lost {
			extra = append(extra, components.Tag(locales.T("results.lost"), "theme.primary"))
		}
	case model.KindLost:
		icon, color = "ghost", "theme.accent"
	case model.KindCarved:
		icon, color = "sparkle", "theme.info"
		title.FontWeight = giode.FontWeightSemiBold
	}
	kids := []giode.Element{icons.Icon(icon, 16, color), giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(n.Title, title))}
	kids = append(kids, extra...)
	return giode.Row(giode.Styles{Width: -1, Height: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Right: theme.S2}}, kids...)
}

func (r *Results) row(i int) giode.Element {
	rows := r.m.Rows()
	if i >= len(rows) {
		return giode.Box(giode.Styles{})
	}
	e := rows[i]
	compact := giode.Compact()
	p := giode.Keyed("row", e, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	return p.Content(func(s giode.PressState) giode.Element {
		return components.FileRow(e, r.m.IsChecked(e), r.m.Preview == e, compact, s)
	}).OnClick(func(ev giode.ClickEvent) {
		if ev.PositionDp.X < components.CheckZone && ev.Source == giode.ClickPointer {
			r.m.Toggle(e)
			return
		}
		if e.IsDir && (ev.Clicks == 2 || compact) {
			giode.AfterLayout(func() { r.m.OpenEntry(e) })
			return
		}
		r.m.Preview = e
		if compact && !e.IsDir {
			r.previewOpen = true
		}
	}).Label(e.Name)
}

func (r *Results) sidebar() giode.Element {
	return giode.Stack(giode.Styles{Width: 264, Height: -1, Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border",
		Padding: giode.Inset{Top: theme.S3, Left: theme.S2, Right: theme.S1}, Gap: theme.S2},
		giode.Box(giode.Styles{Padding: giode.SymmetricInset(theme.S2, theme.S1)}, giode.Text(locales.T("results.found"), theme.Overline())),
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, r.tree),
	)
}
