package screens

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/trash"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

func (d *Dupes) introView() giode.Element {
	st := &d.m.Dupes
	compact := giode.Compact()
	kids := []giode.Element{
		toolHeader("copy", locales.T("dupes.find_duplicates_and_look_alikes"),
			locales.T("dupes.finds_files_that_are_exact_copies")),
	}
	if st.Err != nil {
		kids = append(kids, components.Banner("alert", "theme.danger", st.Err.Error(), nil))
	}
	var chips []giode.Element
	for _, r := range st.Roots {
		r := r
		x := giode.Keyed("dupes-root", r, func() *iconbutton.Button {
			return components.IconAction("x", locales.T("dupes.don_t_look_here")).OnClick(func() { d.m.RemoveDupesRoot(r) })
		})
		chips = append(chips, giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Left: theme.S3, Right: 4},
			Height: 40, Background: theme.Raised(), BorderRadius: 20, BorderWidth: 1, BorderColor: "theme.border"},
			icons.Icon("folder", 16, "theme.primary"),
			giode.Text(r, theme.Body().With(giode.Styles{FontSize: 13, MaxLines: 1})),
			x))
	}
	if len(chips) == 0 {
		chips = append(chips, giode.Text(locales.T("dupes.choose_one_or_more_folders_below"), theme.Secondary()))
	}
	d.start.Disabled(len(st.Roots) == 0)
	options := []giode.Element{d.similar}
	if st.Similar {
		options = append(options, giode.Stack(giode.Styles{Width: -1, Gap: theme.S2},
			giode.Text(locales.T("dupes.how_alike_photos_must_be"), theme.Caption()), d.strict,
			giode.Text(strictNotes[d.strict.Selected()], theme.Caption())))
	}
	kids = append(kids,
		section(locales.T("dupes.look_in"), giode.Row(giode.Styles{Width: -1, Gap: theme.S2}, chips...)),
		section(locales.T("dupes.add_a_folder"), d.chooser.View(compact)),
		section(locales.T("dupes.options"), giode.Stack(giode.Styles{Gap: theme.S3}, options...)),
		giode.Row(giode.Styles{Gap: theme.S3, Align: giode.AlignCenter}, d.start,
			giode.Text(locales.T("dupes.photos_jpeg_png_gif_webp_bmp"), theme.Caption())),
	)
	return page(compact, kids...)
}

func (d *Dupes) running() giode.Element {
	p := d.m.Dupes.Progress
	frac := float32(0)
	if p.Of > 0 {
		frac = float32(p.Done) / float32(p.Of)
	}
	detail := locales.T("dupes.listed", "files", count(p.Files, "file"))
	if p.Of > 0 {
		detail = locales.T("dupes.progress", "done", p.Done, "of", p.Of, "groups", count(p.Groups, "group"))
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: theme.S4, Padding: giode.UniformInset(theme.S6)},
		giode.Spinner(giode.Styles{Width: 36, Height: 36, Color: "theme.primary"}),
		giode.Text(locales.T("common.ongoing", "what", label(p.Phase.Label())), theme.Title()),
		giode.Box(giode.Styles{Width: 360}, giode.Progress(frac, giode.Styles{Height: 6, BorderRadius: 3, Color: "theme.primary", Background: theme.Sunken()})),
		giode.Text(detail, theme.Secondary()),
		d.stop,
	)
}

func (d *Dupes) results() giode.Element {
	st := &d.m.Dupes
	compact := giode.Compact()
	var wasted int64
	for _, g := range st.Groups {
		wasted += g.Wasted()
	}
	summary := locales.T("dupes.summary", "groups", count(len(st.Groups), "group"), "size", recovery.FormatBytes(wasted))
	top := []giode.Element{d.back, giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(summary, theme.Heading().With(giode.Styles{FontSize: 15})))}
	if !compact {
		top = append(top, d.show)
	}
	bar := giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, top...)
	var notes []giode.Element
	if st.Done != "" {
		notes = append(notes, components.Banner("check", "theme.success", st.Done, nil))
	}
	if st.Err != nil {
		notes = append(notes, components.Banner("alert", "theme.danger", st.Err.Error(), nil))
	}
	var body giode.Element
	if len(st.Groups) == 0 {
		body = components.EmptyState("check", locales.T("dupes.no_duplicates_found"), locales.T("dupes.every_file_in_these_folders_is"), nil)
	} else {
		body = giode.Box(giode.Styles{Width: -1, Height: -1, Padding: giode.SymmetricInset(theme.S4, 0)}, d.list)
	}
	n, size := d.m.DupesChecked()
	d.remove.Disabled(n == 0 || st.Removing)
	label := locales.T("dupes.move_to_trash")
	if !trash.Supported {
		label = locales.T("dupes.delete")
	}
	if st.Removing {
		label = locales.T("dupes.removing")
	}
	d.remove.Label(label)
	foot := giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"},
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(locales.T("dupes.checked", "files", count(n, "file"), "size", recovery.FormatBytes(size)), theme.Secondary())),
		d.remove)
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		append(append([]giode.Element{bar}, notes...), giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, body), foot)...)
}
