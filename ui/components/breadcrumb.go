package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/ui/icons"
	"github.com/mehran9675/remanence/ui/model"
	"github.com/mehran9675/remanence/ui/theme"
)

// Breadcrumb shows the path to the open folder; each step opens its
// folder when clicked.
func Breadcrumb(n *model.Node, open func(*model.Node)) giode.Element {
	if n == nil {
		return giode.Box(giode.Styles{})
	}
	trail := n.Trail()
	// Long paths keep the top and the last few steps.
	if len(trail) > 5 {
		trail = append([]*model.Node{trail[0], nil}, trail[len(trail)-3:]...)
	}
	var kids []giode.Element
	for i, c := range trail {
		if i > 0 {
			kids = append(kids, icons.Icon("chevron-right", 14, giode.Mix("theme.text", "theme.surface", 0.3)))
		}
		if c == nil {
			kids = append(kids, giode.Text("…", theme.Secondary()))
			continue
		}
		last := i == len(trail)-1
		c := c
		kids = append(kids, giode.Keyed("crumb", c, func() *giode.PressableElement {
			return giode.Pressable(func(s giode.PressState) giode.Element {
				st := theme.Secondary().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis, MaxWidth: 220})
				if last {
					st = theme.Heading().With(giode.Styles{FontSize: 14, TextOverflow: giode.TextOverflowEllipsis, MaxWidth: 280})
				} else if s.Hovered {
					st.Color = "theme.text"
				}
				return giode.Box(giode.Styles{Padding: giode.SymmetricInset(4, 2), Cursor: giode.CursorPointer}, giode.Text(c.Title, st))
			}).OnClick(func(giode.ClickEvent) { open(c) })
		}))
	}
	return giode.Row(giode.Styles{Gap: 2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, kids...)
}
