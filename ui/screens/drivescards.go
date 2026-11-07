package screens

import (
	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

func (d *Drives) section(title string, action giode.Element, body giode.Element) giode.Element {
	head := []giode.Element{giode.Text(title, theme.Overline().With(giode.Styles{FontSize: 11}))}
	if action != nil {
		head = append(head, action)
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S3},
		giode.Row(giode.Styles{Gap: theme.S2, Align: giode.AlignCenter, Height: 32}, head...),
		body,
	)
}

func (d *Drives) cardWidth(compact bool) giode.Styles {
	if compact {
		return giode.Styles{Width: -1, Height: 112}
	}
	return giode.Styles{Width: 328, Height: 112}
}

func (d *Drives) card(info disk.Info, compact bool) giode.Element {
	p := giode.Keyed("drive", info.ID, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	p.Content(func(s giode.PressState) giode.Element {
		return components.DriveCard(info, info.ID == d.selected, s)
	}).OnClick(func(e giode.ClickEvent) {
		d.selected = info.ID
		if e.Clicks == 2 {
			d.startScan()
		}
	}).Label(info.Name)
	return giode.Box(d.cardWidth(compact), p)
}

func (d *Drives) driveCards(compact bool) giode.Element {
	if d.m.DrivesLoading && len(d.m.Drives) == 0 {
		return giode.Row(giode.Styles{Gap: theme.S3, Align: giode.AlignCenter, Height: 112},
			giode.Spinner(giode.Styles{Width: 18, Height: 18, Color: "theme.primary"}),
			giode.Text(locales.T("drives.looking_for_drives"), theme.Secondary()))
	}
	if len(d.m.Drives) == 0 {
		return components.EmptyState("drive", locales.T("drives.no_drives_found"),
			locales.T("drives.connect_the_drive_you_want_to"), nil)
	}
	var cards []giode.Element
	for _, info := range d.m.Drives {
		if !isBackup(info) && info.Kind != disk.KindVolume {
			cards = append(cards, d.card(info, compact))
		}
	}
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...)
}

func (d *Drives) imageCards(compact bool) giode.Element {
	var cards []giode.Element
	for _, info := range d.images {
		cards = append(cards, d.card(info, compact))
	}
	cards = append(cards, d.openCard("open-image", "plus", locales.T("drives.open_a_disk_image"), locales.T("drives.a_copy_of_a_drive_img"), compact, d.pickImage))
	return giode.Row(giode.Styles{Width: -1, Gap: theme.S3}, cards...)
}

// openCard is the card that opens something not listed: a disk image, a
// backup kept elsewhere.
func (d *Drives) openCard(key, icon, title, detail string, compact bool, open func()) giode.Element {
	p := giode.Keyed("drive", key, func() *giode.PressableElement {
		return giode.Pressable(func(giode.PressState) giode.Element { return nil })
	})
	p.Content(func(s giode.PressState) giode.Element {
		border := "theme.border"
		if s.Hovered {
			border = "theme.primary"
		}
		return giode.Row(giode.Styles{Width: -1, Height: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
			Padding: giode.UniformInset(theme.S4), BorderRadius: theme.RadiusLarge, BorderWidth: 1, BorderColor: border,
			Cursor: giode.CursorPointer},
			giode.Box(giode.Styles{Width: 44, Height: 44, BorderRadius: 12, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
				Background: giode.Mix("theme.primary", "theme.surface", 0.12)}, icons.Icon(icon, 20, "theme.primary")),
			giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
				giode.Text(title, theme.Heading()),
				giode.Text(detail, theme.Secondary()),
			),
		)
	}).OnClick(func(giode.ClickEvent) { open() }).Label(title)
	return giode.Box(d.cardWidth(compact), p)
}
