package screens

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

func (r *Rename) introView() giode.Element {
	compact := giode.Compact()
	kids := []giode.Element{
		toolHeader("rename", locales.T("rename.rename_many_files_at_once"),
			locales.T("rename.number_photos_put_the_date_they")),
	}
	if err := r.m.Rename.Err; err != nil {
		kids = append(kids, components.Banner("alert", "theme.danger", err.Error(), nil))
	}
	kids = append(kids, section(locales.T("rename.choose_the_folder"), r.chooser.View(compact)))
	return page(compact, kids...)
}

func (r *Rename) rulesView() giode.Element {
	label := func(s string) giode.Element {
		return giode.Text(s, theme.Caption().With(giode.Styles{FontWeight: giode.FontWeightMedium}))
	}
	var chips []giode.Element
	for _, c := range r.chips {
		chips = append(chips, c)
	}
	ext := []giode.Element{r.extSel}
	if r.extSel.Selected() == 2 {
		ext = append(ext, giode.Box(giode.Styles{Width: 110}, r.ext))
	}
	kids := []giode.Element{
		section(locales.T("rename.pattern"),
			r.pattern,
			giode.Row(giode.Styles{Width: -1, Gap: 6}, chips...),
			giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				label(locales.T("rename.numbers_start_at")), giode.Box(giode.Styles{Width: 72}, r.start),
				label("by"), giode.Box(giode.Styles{Width: 60}, r.step)),
			giode.Text("{date} is when a photo was taken, or when the file last changed; {date:2006-01-02 15.04} sets its layout.", theme.Caption().With(giode.Styles{FontSize: 11.5})),
		),
		section(locales.T("rename.find_and_replace"), r.find, r.replace, r.regexp, r.matchCase, r.withExt),
		section(locales.T("rename.case_and_extension"), r.caseSel, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, ext...)),
		section(locales.T("rename.files"), r.recursive, r.order),
		r.reset,
	}
	if err := r.m.Rename.PlanErr; err != nil {
		kids = append([]giode.Element{components.Banner("alert", "theme.danger", err.Error(), nil)}, kids...)
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S5, Padding: giode.UniformInset(theme.S4)}, kids...)
}

func (r *Rename) editor() giode.Element {
	st := &r.m.Rename
	compact := giode.Compact()
	changes, problems := r.m.RenameCounts()
	bar := giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"},
		r.back, icons.Icon("folder", 16, "theme.primary"),
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(st.Dir, theme.Body().With(giode.Styles{FontSize: 13, FontWeight: giode.FontWeightMedium, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis}))),
		giode.Text(count(len(st.Files), "file"), theme.Caption()),
		r.again,
	)
	var notes []giode.Element
	if st.Done != "" {
		notes = append(notes, components.Banner("check", "theme.success", st.Done, nil))
	}
	if st.Err != nil {
		notes = append(notes, components.Banner("alert", "theme.danger", st.Err.Error(), nil))
	}
	var preview giode.Element = giode.Box(giode.Styles{Width: -1, Height: -1, Padding: giode.SymmetricInset(theme.S3, theme.S2)}, r.list)
	if len(st.Files) == 0 {
		preview = components.EmptyState("folder", locales.T("rename.no_files_here"), locales.T("rename.this_folder_has_no_files_to"), nil)
	}
	head := giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Padding: giode.Inset{Left: theme.S6 + 30, Right: theme.S4, Top: theme.S3, Bottom: theme.S1}},
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(locales.T("rename.now"), theme.Overline())),
		giode.Box(giode.Styles{Width: 14}),
		giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(locales.T("rename.after"), theme.Overline())),
	)
	summary := locales.N("rename.names_change", changes)
	if problems > 0 {
		summary = locales.T("rename.with_problems", "changes", summary, "problems", count(problems, "problem"))
	}
	r.apply.Disabled(changes == 0 || problems > 0 || st.PlanErr != nil)
	foot := []giode.Element{giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(summary, theme.Secondary()))}
	if len(st.Undo) > 0 {
		foot = append(foot, r.undo)
	}
	foot = append(foot, r.apply)
	footer := giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S4, theme.S3),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, foot...)
	if compact {
		return giode.Stack(giode.Styles{Width: -1, Height: -1}, append(append([]giode.Element{bar}, notes...),
			giode.Box(giode.Styles{Width: -1, Height: 300, BorderWidth: 1, BorderColor: "theme.border"}, r.rules),
			head,
			giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, preview),
			footer)...)
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1}, append(append([]giode.Element{bar}, notes...),
		giode.Row(giode.Styles{Width: -1, FlexGrow: 1, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 360, Height: -1, Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, r.rules),
			giode.Stack(giode.Styles{FlexGrow: 1, Height: -1}, head, giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, preview)),
		),
		footer)...)
}
