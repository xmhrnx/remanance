package screens

import (
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

// View lays out the screen.
func (r *Rename) View() giode.Element {
	if r.m.Rename.Dir == "" {
		return r.intro
	}
	return r.editor()
}
