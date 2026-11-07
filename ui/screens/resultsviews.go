package screens

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/engine/scan"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// summary is the line along the top: what was scanned and what was found.
func (r *Results) summary(compact bool) giode.Element {
	res := r.m.Result
	name := r.m.SourceName()
	files, deleted, carved := 0, 0, 0
	if res != nil {
		files, deleted, carved = res.Stats()
	}
	facts := locales.T("results.facts", "files", files, "deleted", deleted, "carved", carved)
	if res != nil && res.Duration > 0 {
		facts = locales.T("results.facts_scanned", "facts", facts, "time", friendly(res.Duration))
	}
	st := giode.Styles{Width: -1, Gap: theme.S4, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S5, theme.S3), BorderWidth: 1, BorderColor: "theme.border", Background: "theme.background"}
	left := giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
		giode.Text(name, theme.Heading().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
		giode.Text(facts, theme.Caption().With(giode.Styles{FontWeight: giode.FontWeightNormal})),
	)
	kids := []giode.Element{r.newScan, left}
	if disk.IsGone(r.m.ScanErr) {
		kids = append(kids, components.Tag(locales.T("results.drive_disconnected"), "theme.danger"))
	} else if r.m.ScanErr != nil {
		kids = append(kids, components.Tag(locales.T("results.stopped_early"), "theme.warning"))
	}
	if compact {
		st.Padding = giode.SymmetricInset(theme.S3, theme.S2)
	}
	return giode.Row(st, kids...)
}

func (r *Results) table(compact bool) giode.Element {
	bar := []giode.Element{}
	if compact && r.m.Folder != nil && r.m.Folder.Parent != nil {
		bar = append(bar, r.up)
	}
	bar = append(bar, giode.Box(giode.Styles{FlexGrow: 1}, components.Breadcrumb(r.m.Folder, r.m.OpenFolder)))
	if !compact {
		bar = append(bar, r.show, giode.Box(giode.Styles{Width: 200}, r.search))
	}
	var filters []giode.Element
	if compact {
		filters = append(filters, r.show)
	}
	for i, c := range r.cats {
		filters = append(filters, c.View(r.m.Filter.Category == r.catValues[i]))
	}
	var head []giode.Element
	head = append(head,
		giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, bar...),
	)
	if compact {
		// No header to click on a phone: a menu sorts instead.
		for i, c := range model.SortChoices {
			if c.Order == r.m.Order && r.sort.Selected() != i {
				r.sort.SetSelected(i)
			}
		}
		head = append(head, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{FlexGrow: 1}, r.search),
			giode.Box(giode.Styles{Width: 168}, r.sort)))
	}
	head = append(head, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter}, filters...))

	rows := r.m.Rows()
	var list giode.Element = r.list
	if len(rows) == 0 {
		list = r.empty()
	}
	parts := []giode.Element{
		giode.Stack(giode.Styles{Width: -1, Gap: theme.S3, Padding: giode.Inset{Top: theme.S4, Left: theme.S5, Right: theme.S5, Bottom: theme.S3}}, head...),
	}
	if !compact {
		parts = append(parts, giode.Box(giode.Styles{Width: -1, BorderWidth: 1, BorderColor: theme.Hairline()}, components.FileHeader(r.checkAll, r.m.Order, r.m.SortBy)))
	}
	if r.m.Searching() && len(rows) > 0 {
		parts = append(parts, giode.Box(giode.Styles{Padding: giode.SymmetricInset(theme.S5, 6)},
			giode.Text(locales.T("results.matches", "matches", count(len(rows), "match")), theme.Caption())))
	}
	parts = append(parts, giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, list))
	return giode.Stack(giode.Styles{Width: -1, Height: -1}, parts...)
}

func (r *Results) empty() giode.Element {
	switch {
	case r.m.Folder == nil && r.m.Mode == scan.Quick:
		return components.EmptyState("search", locales.T("results.no_file_system_could_be_read"),
			locales.T("results.the_drive_may_have_been_formatted"), r.deep)
	case r.m.Folder == nil:
		return components.EmptyState("search", locales.T("results.nothing_was_found"),
			locales.T("results.every_sector_was_read_but_nothing"), nil)
	case r.m.Searching():
		return components.EmptyState("search", locales.T("results.no_matches"), locales.T("results.nothing_here_matches_the_search_and"), nil)
	case r.m.Filter.Show == model.ShowDeleted:
		return components.EmptyState("check", locales.T("results.no_deleted_files_here"),
			locales.T("results.this_folder_has_no_deleted_files"), nil)
	}
	return components.EmptyState("folder-open", locales.T("results.this_folder_is_empty"), "", nil)
}

// bar is the strip along the bottom: the selection and the recover button.
func (r *Results) bar(files int, size int64, compact bool) giode.Element {
	msg := locales.T("results.check_the_files_and_folders_to")
	if files > 0 {
		msg = locales.T("results.selected", "files", count(files, "file"), "size", recovery.FormatBytes(size))
	}
	kids := []giode.Element{giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(msg, theme.Secondary()))}
	if files > 0 {
		kids = append(kids, r.clear)
	}
	kids = append(kids, r.recover)
	pad := giode.SymmetricInset(theme.S5, theme.S3)
	if compact {
		pad = giode.UniformInset(theme.S3)
	}
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: pad, Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"}, kids...)
}
