package screens

import (
	"slices"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/platform"
	"github.com/xmhrnx/remanance/ui/theme"
)

func (d *Drives) content() giode.Element {
	compact := giode.Compact()
	pad := theme.S8
	if compact {
		pad = theme.S4
	}
	var notes []giode.Element
	if d.m.NeedsAdmin && !giode.IsElevated() {
		d.elevate.Disabled(d.elevating)
		if d.elevating {
			d.elevate.Label(locales.T("drives.waiting_for_permission"))
		} else {
			d.elevate.Label(locales.T("drives.restart_as_administrator"))
		}
		notes = append(notes, components.Banner("lock", "theme.warning",
			locales.T("drives.reading_drives_directly_needs_administrator_rights"), d.elevate))
	}
	if needsPassword(d.m.OpenErr) {
		d.unlock.Disabled(d.m.Opening)
		if d.m.Opening {
			d.unlock.Label(locales.T("drives.unlocking"))
		} else {
			d.unlock.Label(locales.T("drives.unlock"))
		}
		notes = append(notes, components.Banner("lock", "theme.warning", d.m.OpenErr.Error(),
			giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{Width: 220}, d.password), d.unlock)))
	}
	for _, err := range []error{d.m.OpenErr, d.m.DrivesErr, d.pickErr} {
		if err != nil && !(d.m.NeedsAdmin && err == d.m.OpenErr) && !needsPassword(err) {
			notes = append(notes, components.Banner("alert", "theme.danger", err.Error(), nil))
		}
	}
	return giode.Box(giode.Styles{Width: -1, Justify: giode.JustifyCenter},
		giode.Stack(giode.Styles{Width: -1, MaxWidth: 1080, Padding: giode.UniformInset(pad), Gap: theme.S6},
			giode.Stack(giode.Styles{Gap: theme.S2},
				giode.Text(locales.T("drives.choose_where_to_look"), theme.Hero()),
				giode.Box(giode.Styles{MaxWidth: 620}, giode.Text(
					locales.T("drives.datadoc_reads_the_drive_without_changing"),
					theme.Secondary().With(giode.Styles{FontSize: 14, LineHeight: 21}))),
			),
			giode.Stack(giode.Styles{Gap: theme.S3}, notes...),
			d.section(locales.T("drives.drives"), d.refresh, d.driveCards(compact)),
			d.partSection(compact),
			d.section(locales.T("drives.disk_images"), nil, d.imageCards(compact)),
			d.backupSection(compact),
		),
	)
}

// partSection lists the drives' partitions, such as C: and D:, to scan
// one on its own, and offers to scan only a folder on one. Both read
// only a file system, which is quicker than a whole drive.
func (d *Drives) partSection(compact bool) giode.Element {
	var cards []giode.Element
	for _, info := range d.m.Drives {
		if info.Kind == disk.KindVolume {
			cards = append(cards, d.card(info, compact))
		}
	}
	if len(cards) == 0 {
		return giode.Box(giode.Styles{})
	}
	if platform.PicksFolders {
		cards = append(cards, d.openCard("scan-folder", "folder", locales.T("drives.scan_a_folder"), locales.T("drives.only_what_was_deleted_from_it"), compact, d.pickFolder))
	}
	return d.section(locales.T("drives.partitions_and_folders"), nil, giode.Stack(giode.Styles{Width: -1, Gap: theme.S3},
		giode.Box(giode.Styles{MaxWidth: 620}, giode.Text(locales.T("drives.partitions_note"), theme.Secondary())),
		giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...)))
}

// backupSection lists the iPhone and iPad backups on this computer, on
// desktops, which hold photos and app data the phone has since lost.
func (d *Drives) backupSection(compact bool) giode.Element {
	if !platform.ReadsBackups {
		return giode.Box(giode.Styles{})
	}
	var cards []giode.Element
	for _, info := range slices.Concat(d.m.Drives, d.backups) {
		if isBackup(info) {
			cards = append(cards, d.card(info, compact))
		}
	}
	cards = append(cards, d.openCard("open-backup", "phone", locales.T("drives.open_an_iphone_backup"), locales.T("drives.a_backup_folder_made_by_finder"), compact, d.pickBackup))
	return d.section(locales.T("drives.iphone_and_ipad_backups"), nil, giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...))
}

// bar is the strip along the bottom: the scan mode and the start button.
func (d *Drives) bar() giode.Element {
	hint := locales.T("drives.reads_the_file_systems_for_deleted")
	if d.mode.Selected() == 1 {
		hint = locales.T("drives.also_reads_every_sector_finds_lost")
	}
	compact := giode.Compact()
	inner := []giode.Element{
		giode.Stack(giode.Styles{Gap: 6, FlexGrow: 1},
			d.mode,
			giode.Text(hint, theme.Caption().With(giode.Styles{FontWeight: giode.FontWeightNormal})),
		),
		d.start,
	}
	st := giode.Styles{Width: -1, Gap: theme.S4, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Padding: giode.SymmetricInset(theme.S8, theme.S4), Background: "theme.surface",
		BorderWidth: 1, BorderColor: "theme.border"}
	if compact {
		st.Padding = giode.UniformInset(theme.S4)
		st.Wrap = giode.WrapWrap
	}
	return giode.Row(st, inner...)
}
