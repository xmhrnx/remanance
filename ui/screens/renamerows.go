package screens

import (
	"path/filepath"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/rename"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

func (r *Rename) row(i int) giode.Element {
	plan := r.m.Rename.Plan
	if i >= len(plan) {
		return giode.Box(giode.Styles{})
	}
	c := plan[i]
	p := giode.Keyed("rename-row", c.Path, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	return p.Content(func(s giode.PressState) giode.Element {
		return r.changeRow(c, s)
	}).OnClick(func(giode.ClickEvent) { r.m.ToggleRenameFile(c.Path) }).Label(filepath.Base(c.Path))
}

func (r *Rename) changeRow(c rename.Change, s giode.PressState) giode.Element {
	bg := "transparent"
	if s.Hovered {
		bg = theme.Raised()
	}
	old := filepath.Base(c.Path)
	if r.m.Rename.Recursive {
		if rel, err := filepath.Rel(r.m.Rename.Dir, c.Path); err == nil {
			old = rel
		}
	}
	newStyle := theme.Body().With(giode.Styles{FontSize: 13, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis})
	switch {
	case c.Problem != "":
		newStyle.Color = "theme.danger"
	case c.Same():
		newStyle.Color = "theme.muted"
	default:
		newStyle.Color, newStyle.FontWeight = "theme.text", giode.FontWeightMedium
	}
	newCol := []giode.Element{giode.Text(c.New, newStyle)}
	if c.Problem != "" {
		newCol = append(newCol, giode.Text(c.Problem, theme.Caption().With(giode.Styles{FontSize: 11, Color: "theme.danger", MaxLines: 1})))
	}
	return giode.Row(giode.Styles{Width: -1, Height: renameRowHeight, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S3, 0), Background: bg, BorderRadius: theme.RadiusSmall},
		components.CheckMark(true, s.Hovered),
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(old, theme.Secondary().With(giode.Styles{FontSize: 13, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis}))),
		icons.Icon("arrow-right", 14, "theme.muted"),
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 1}, newCol...),
	)
}
