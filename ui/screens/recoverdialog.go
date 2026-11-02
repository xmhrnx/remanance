package screens

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/checkbox"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/input"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/platform"
	"github.com/xmhrnx/remanance/ui/theme"
)

// RecoverDialog asks where to save the checked files, copies them and
// says how it went.
type RecoverDialog struct {
	m    *model.Model
	open bool
	dlg  *dialog.Dialog

	dest    *input.Input
	choose  *giode.ButtonElement
	folders *checkbox.Checkbox
	start   *giode.ButtonElement
	cancel  *giode.ButtonElement
	stop    *giode.ButtonElement
	reveal  *giode.ButtonElement
	done    *giode.ButtonElement
}

// NewRecoverDialog builds the dialog, closed.
func NewRecoverDialog(m *model.Model) *RecoverDialog {
	d := &RecoverDialog{m: m}
	d.dlg = giode.Dialog(&d.open, d.content, giode.Styles{Width: 520, BorderRadius: theme.RadiusLarge, Padding: giode.UniformInset(theme.S6),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"})
	d.dest = giode.Input.Text(locales.T("recoverdialog.folder_to_save_into"), giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 9), BorderRadius: theme.Radius})
	d.dest.SetText(platform.DefaultDestination())
	d.choose = components.SecondaryButton(locales.T("recoverdialog.choose"), "folder").OnClick(func() {
		opts := giode.FolderOptions{Title: locales.T("recoverdialog.where_should_the_recovered_files_go"), Directory: d.dest.Text()}
		go func() {
			if dir, err := m.App.PickFolder(context.Background(), opts); err == nil {
				m.App.Post(func() { d.dest.SetText(dir) })
			}
		}()
	})
	d.folders = giode.Input.Checkbox(locales.T("recoverdialog.keep_the_folders_the_files_were"), giode.Styles{FontSize: 13, Color: "theme.text", Background: "theme.primary", BorderColor: "theme.border"})
	d.folders.SetChecked(true)
	d.start = components.PrimaryButton(locales.T("recoverdialog.recover"), "recover").OnClick(d.begin)
	d.cancel = components.GhostButton(locales.T("recoverdialog.cancel"), "").OnClick(func() { d.open = false })
	d.stop = components.SecondaryButton(locales.T("recoverdialog.stop"), "stop").OnClick(m.CancelRecover)
	d.reveal = components.SecondaryButton(locales.T("recoverdialog.show_the_files"), "external").OnClick(d.show)
	d.done = components.PrimaryButton(locales.T("recoverdialog.done"), "check").OnClick(func() {
		d.open = false
		m.Report, m.RecoverErr = nil, nil
	})
	return d
}

// Open shows the dialog for the checked files.
func (d *RecoverDialog) Open() {
	if !d.m.Recovering {
		d.m.Report, d.m.RecoverErr = nil, nil
	}
	d.open = true
}

func (d *RecoverDialog) begin() {
	dir := d.dest.Text()
	if platform.ChooseDestination && d.m.CheckDestination(dir) != nil {
		return
	}
	sink, err := platform.Sink(dir)
	if err != nil {
		d.m.RecoverErr = err
		return
	}
	d.m.StartRecover(sink, d.folders.Checked())
}

func (d *RecoverDialog) show() {
	if d.m.Report == nil {
		return
	}
	target := filepath.Join(d.m.Report.Location, "Recovery report.txt")
	if _, err := os.Stat(target); err != nil {
		target = d.m.Report.Location
	}
	go d.m.App.Share(context.Background(), giode.ShareFile{Path: target})
}

// View places the dialog; it takes no space while closed.
func (d *RecoverDialog) View() giode.Element {
	// It stays open while files are being copied.
	d.dlg.Dismissible(!d.m.Recovering)
	return d.dlg
}

func (d *RecoverDialog) content() giode.Element {
	switch {
	case d.m.Recovering:
		return d.running()
	case d.m.Report != nil:
		return d.finished()
	}
	return d.setup()
}

func (d *RecoverDialog) header(icon, color, title, sub string) giode.Element {
	return giode.Row(giode.Styles{Gap: theme.S4, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		giode.Box(giode.Styles{Width: 44, Height: 44, BorderRadius: 12, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
			Background: giode.Mix(color, "theme.surface", 0.14)}, icons.Icon(icon, 22, color)),
		giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
			giode.Text(title, theme.Title().With(giode.Styles{FontSize: 19})),
			giode.Text(sub, theme.Secondary()),
		),
	)
}

func (d *RecoverDialog) setup() giode.Element {
	files, size := d.m.Selection()
	parts := []giode.Element{
		d.header("recover", "theme.primary", locales.T("recoverdialog.title", "files", count(files, "file")), locales.T("recoverdialog.to_copy", "size", recovery.FormatBytes(size))),
	}
	if platform.ChooseDestination {
		dir := d.dest.Text()
		parts = append(parts, giode.Stack(giode.Styles{Gap: theme.S2},
			giode.Text(locales.T("recoverdialog.save_into"), theme.Overline()),
			giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1}, d.dest), d.choose),
		), d.folders)
		err := d.m.CheckDestination(dir)
		d.start.Disabled(err != nil || files == 0)
		switch {
		case err != nil && dir != "":
			parts = append(parts, components.Banner("alert", "theme.danger", capitalize(err.Error()), nil))
		case err != nil:
		case d.m.Source.Kind == disk.KindImage:
			parts = append(parts, components.Banner("shield", "theme.success",
				locales.T("recoverdialog.recovering_from_a_disk_image_only"), nil))
		default:
			parts = append(parts, components.Banner("shield", "theme.success",
				locales.T("recoverdialog.this_folder_is_on_another_drive"), nil))
		}
	} else {
		d.start.Disabled(files == 0)
		parts = append(parts, d.folders, components.Banner("folder", "theme.info", platform.DestinationNote(), nil))
	}
	if d.m.RecoverErr != nil {
		parts = append(parts, components.Banner("alert", "theme.danger", d.m.RecoverErr.Error(), nil))
	}
	parts = append(parts, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Justify: giode.JustifyEnd}, d.cancel, d.start))
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S5}, parts...)
}

func (d *RecoverDialog) running() giode.Element {
	p := d.m.RecoverP
	frac := float32(0)
	if p.Total > 0 {
		frac = float32(float64(p.Done) / float64(p.Total))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S5},
		d.header("recover", "theme.primary", locales.T("recoverdialog.recovering"), locales.T("recoverdialog.progress", "done", p.Files, "files", p.TotalFiles,
			"copied", recovery.FormatBytes(p.Done), "size", recovery.FormatBytes(p.Total))),
		giode.Progress(frac, giode.Styles{Width: -1, Height: 8, BorderRadius: 4, Color: "theme.primary",
			Background: giode.Mix("theme.text", "theme.surface", 0.08)}),
		giode.Text(p.Current, theme.Code().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
		giode.Row(giode.Styles{Width: -1, Justify: giode.JustifyEnd}, d.stop),
	)
}

func (d *RecoverDialog) finished() giode.Element {
	rep := d.m.Report
	icon, color, title := "check", "theme.success", locales.T("recoverdialog.recovered", "files", count(rep.Recovered, "file"))
	if rep.Recovered == 0 {
		icon, color, title = "alert", "theme.danger", locales.T("recoverdialog.nothing_was_recovered")
	}
	parts := []giode.Element{
		d.header(icon, color, title, locales.T("recoverdialog.saved_in", "size", recovery.FormatBytes(rep.Bytes), "time", friendly(rep.Duration))),
		giode.Stack(giode.Styles{Gap: theme.S1},
			giode.Text(locales.T("recoverdialog.saved_into"), theme.Overline()),
			giode.Text(rep.Location, theme.Code().With(giode.Styles{Color: "theme.text"})).Selectable("recovered-location"),
		),
	}
	if rep.Failed > 0 {
		parts = append(parts, components.Banner("alert", "theme.warning",
			locales.T("recoverdialog.failed", "files", count(rep.Failed, "file")), nil))
	}
	if d.m.RecoverErr != nil {
		parts = append(parts, components.Banner("info", "theme.info", capitalize(d.m.RecoverErr.Error())+".", nil))
	}
	parts = append(parts, giode.Text(locales.T("recoverdialog.files_marked_fair_or_poor_may"), theme.Secondary()))
	buttons := []giode.Element{}
	if platform.CanReveal {
		buttons = append(buttons, d.reveal)
	}
	buttons = append(buttons, d.done)
	parts = append(parts, giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Justify: giode.JustifyEnd}, buttons...))
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S5}, parts...)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
