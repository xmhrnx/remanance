package screens

import (
	"path/filepath"
	"strconv"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dropdown"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/segmented"
	"github.com/mehran9675/giode/components/toggle"
	"github.com/xmhrnx/remanance/engine/rename"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// renameRowHeight is the height of a row in the preview.
const renameRowHeight = 44

// tokens are the fields a pattern can hold, offered as chips.
var tokens = []struct{ label, text string }{
	{locales.T("rename.name"), "{name}"}, {locales.T("rename.number"), "{n:3}"}, {locales.T("rename.date_taken"), "{date}"}, {locales.T("rename.folder"), "{parent}"}, {locales.T("rename.extension"), "{ext}"},
}

// Rename renames many files at once by rules, showing every new name
// before anything changes.
type Rename struct {
	m       *model.Model
	chooser *FolderChooser
	intro   *scroll.Scroll
	rules   *scroll.Scroll
	list    *giode.VirtualListElement

	pattern   *input.Input
	start     *input.Input
	step      *input.Input
	find      *input.Input
	replace   *input.Input
	ext       *input.Input
	regexp    *toggle.Switch
	matchCase *toggle.Switch
	withExt   *toggle.Switch
	recursive *toggle.Switch
	caseSel   *dropdown.Select
	extSel    *dropdown.Select
	order     *segmented.Control
	chips     []*giode.PressableElement

	back  *iconbutton.Button
	again *iconbutton.Button
	apply *giode.ButtonElement
	undo  *giode.ButtonElement
	reset *giode.ButtonElement
}

// NewRename builds the renamer.
func NewRename(m *model.Model) *Rename {
	r := &Rename{m: m}
	r.chooser = NewFolderChooser(m, "rename-place", locales.T("rename.open"), m.OpenRenameFolder)
	r.intro = giode.Scroll(r.introView, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	r.rules = giode.Scroll(r.rulesView, giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	r.list = giode.VirtualList(func() int { return len(m.Rename.Plan) }, renameRowHeight, r.row,
		giode.Styles{ScrollBar: giode.ScrollBar{AutoHide: true}})
	field := func(placeholder string) *input.Input {
		return giode.Input.Text(placeholder, giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 8), BorderRadius: theme.Radius}).
			OnChange(func(string) { r.update() })
	}
	r.pattern = field("{name}")
	r.pattern.SetText("{name}")
	r.start = field("1").Keyboard(input.KeyboardNumeric)
	r.start.SetText("1")
	r.step = field("1").Keyboard(input.KeyboardNumeric)
	r.step.SetText("1")
	r.find = field(locales.T("rename.find"))
	r.replace = field(locales.T("rename.replace_with"))
	r.ext = field("jpg")
	sw := func(label string) *toggle.Switch {
		return giode.Input.Switch(label, giode.Styles{FontSize: 13, Color: "theme.text", Background: "theme.primary"}).OnChange(func(bool) { r.update() })
	}
	r.regexp, r.matchCase, r.withExt = sw(locales.T("rename.regular_expression")), sw(locales.T("rename.match_case")), sw(locales.T("rename.include_the_extension"))
	r.recursive = giode.Input.Switch(locales.T("rename.include_subfolders"), giode.Styles{FontSize: 13, Color: "theme.text", Background: "theme.primary"}).
		OnChange(func(on bool) { m.Rename.Recursive = on; m.ListRenameFiles() })
	r.caseSel = giode.Input.Select(giode.Styles{FontSize: 13, BorderRadius: theme.Radius}, labels(rename.CaseLabels)...).Sheet(giode.SheetCompact).
		OnChange(func(int) { r.update() })
	r.extSel = giode.Input.Select(giode.Styles{FontSize: 13, BorderRadius: theme.Radius}, locales.T("rename.keep_extension"), "lowercase", locales.T("rename.change_to")).Sheet(giode.SheetCompact).
		OnChange(func(int) { r.update() })
	r.order = giode.Input.Segmented(locales.T("rename.by_name"), locales.T("rename.by_date")).Styles(segmentStyles().With(giode.Styles{Width: 200, FontSize: 12.5})).
		OnChange(func(i int) { m.Rename.Order = model.RenameOrder(i); m.ListRenameFiles() })
	for _, t := range tokens {
		t := t
		r.chips = append(r.chips, giode.Pressable(func(s giode.PressState) giode.Element {
			bg := theme.Raised()
			if s.Hovered {
				bg = giode.Mix("theme.primary", "theme.surface", 0.16)
			}
			return giode.Row(giode.Styles{Gap: 4, Align: giode.AlignCenter, Padding: giode.SymmetricInset(10, 5), Background: bg, BorderRadius: 14,
				BorderWidth: 1, BorderColor: "theme.border"},
				icons.Icon("plus", 12, "theme.primary"), giode.Text(t.label, giode.Styles{FontSize: 12, Color: "theme.text"}))
		}).OnClick(func(giode.ClickEvent) {
			r.pattern.InsertText(t.text)
			r.update()
		}).Label(locales.T("rename.insert_token", "token", t.label)))
	}
	r.back = components.IconAction("arrow-left", locales.T("rename.choose_another_folder")).OnClick(m.CloseRename)
	r.again = components.IconAction("refresh", locales.T("rename.read_the_folder_again")).OnClick(m.ListRenameFiles)
	r.apply = components.PrimaryButton(locales.T("rename.rename"), "check").OnClick(m.ApplyRename)
	r.undo = components.SecondaryButton(locales.T("rename.undo"), "history").OnClick(m.UndoRename)
	r.reset = components.GhostButton(locales.T("rename.reset_rules"), "").OnClick(func() {
		r.pattern.SetText("{name}")
		r.start.SetText("1")
		r.step.SetText("1")
		r.find.SetText("")
		r.replace.SetText("")
		r.ext.SetText("")
		r.regexp.SetChecked(false)
		r.matchCase.SetChecked(false)
		r.withExt.SetChecked(false)
		r.caseSel.SetSelected(0)
		r.extSel.SetSelected(0)
		r.update()
	})
	return r
}

// update builds the rules from the fields and works out the names.
func (r *Rename) update() {
	start, err := strconv.Atoi(r.start.Text())
	if err != nil {
		start = 1
	}
	step, err := strconv.Atoi(r.step.Text())
	if err != nil || step == 0 {
		step = 1
	}
	rules := rename.Rules{
		Pattern: r.pattern.Text(), Start: start, Step: step,
		Find: r.find.Text(), Replace: r.replace.Text(), Regexp: r.regexp.Checked(), MatchCase: r.matchCase.Checked(),
		Case: rename.Case(r.caseSel.Selected()), WithExt: r.withExt.Checked(),
	}
	switch r.extSel.Selected() {
	case 1:
		rules.LowerExt = true
	case 2:
		rules.Ext = r.ext.Text()
	}
	r.m.SetRenameRules(rules)
}

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

// View lays out the screen.
func (r *Rename) View() giode.Element {
	if r.m.Rename.Dir == "" {
		return r.intro
	}
	return r.editor()
}

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
