package components

import (
	"fmt"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

func (p *Preview) body(e *fsys.Entry) giode.Element {
	if e == nil {
		return giode.Stack(giode.Styles{Width: -1, Padding: giode.UniformInset(theme.S6), Gap: theme.S3, Align: giode.AlignCenter},
			giode.Box(giode.Styles{Height: 60}),
			icons.Icon("eye", 28, giode.Mix("theme.text", "theme.surface", 0.3)),
			giode.Text(locales.T("preview.select_a_file_to_preview_it"), theme.Secondary().With(giode.Styles{TextAlign: giode.TextAlignCenter})),
		)
	}
	parts := []giode.Element{p.thumbnail(e),
		giode.Stack(giode.Styles{Gap: theme.S2},
			giode.Text(e.Name, theme.Heading().With(giode.Styles{MaxLines: 3})),
			giode.Row(giode.Styles{Gap: 6}, StatePill(e.State)),
		),
		p.details(e),
	}
	if e.Note != "" {
		parts = append(parts, Banner("info", "theme.info", e.Note, nil))
	}
	if !e.IsDir && len(p.head) > 0 {
		label := locales.T("preview.first_bytes")
		var body giode.Element
		if LooksLikeText(p.head) {
			label = locales.T("preview.beginning_of_the_text")
			body = giode.Text(strings.ToValidUTF8(string(p.head), "�"), theme.Code().With(giode.Styles{FontSize: 11.5, LineHeight: 17, Color: "theme.text", MaxLines: 40})).Selectable("previewtext")
		} else {
			body = HexView(p.head[:min(len(p.head), 512)])
		}
		parts = append(parts, giode.Stack(giode.Styles{Gap: theme.S2},
			giode.Text(label, theme.Overline()),
			giode.Box(giode.Styles{Width: -1, Padding: giode.UniformInset(theme.S3), Background: theme.Sunken(), BorderRadius: theme.Radius}, body),
		))
	}
	if p.err != nil {
		parts = append(parts, Banner("alert", "theme.danger", locales.T("preview.couldnt_read", "error", p.err.Error()), nil))
	}
	return giode.Stack(giode.Styles{Width: -1, Padding: giode.UniformInset(theme.S4), Gap: theme.S4}, parts...)
}

func (p *Preview) thumbnail(e *fsys.Entry) giode.Element {
	box := giode.Styles{Width: -1, Height: 200, BorderRadius: theme.Radius, Background: theme.Sunken(),
		Align: giode.AlignCenter, Justify: giode.JustifyCenter, Overflow: giode.OverflowHidden}
	switch {
	case p.img != nil:
		return giode.Box(box, giode.Image(p.img, giode.Styles{Width: -1, Height: -1, Fit: giode.FitContain}))
	case p.loading && model.CategoryOf(e) == model.CatPictures:
		return giode.Box(box, giode.Spinner(giode.Styles{Width: 22, Height: 22, Color: "theme.primary"}))
	}
	return giode.Box(box, FileIcon(e, 44))
}

func (p *Preview) details(e *fsys.Entry) giode.Element {
	type kv struct{ k, v string }
	var rows []kv
	if !e.IsDir {
		rows = append(rows, kv{locales.T("preview.size"), locales.T("preview.size_bytes", "size", recovery.FormatBytes(e.Size), "bytes", e.Size)})
	}
	if e.State != fsys.Existing && !e.IsDir {
		rows = append(rows, kv{locales.T("preview.recovery_chance"), fmt.Sprintf("%s, %d%%", HealthLabel(e.Health), e.Health)})
	}
	rows = append(rows, kv{locales.T("preview.modified"), FormatTime(e.Modified)}, kv{locales.T("preview.created"), FormatTime(e.Created)})
	if len(e.Runs) > 0 && e.Runs[0].Offset >= 0 {
		frag := ""
		if len(e.Runs) > 1 {
			frag = locales.T("preview.fragments", "n", len(e.Runs))
		}
		rows = append(rows, kv{locales.T("preview.location"), locales.T("preview.sector", "sector", e.Runs[0].Offset/512, "fragments", frag)})
	}
	rows = append(rows, kv{locales.T("preview.path"), e.Path()})
	var out []giode.Element
	for _, r := range rows {
		out = append(out, giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 108}, giode.Text(r.k, theme.Secondary())),
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(r.v, theme.Body().With(giode.Styles{FontSize: 13}))),
		))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S2}, out...)
}
