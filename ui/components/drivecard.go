package components

import (
	"strings"

	"github.com/mehran9675/giode"
	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/recovery"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

// DriveIcon picks the icon for a drive from how it's attached.
func DriveIcon(info disk.Info) string {
	bus := strings.ToLower(info.Bus)
	switch {
	case info.Kind == disk.KindImage:
		return "disc"
	case info.Kind == disk.KindFiles, bus == locales.T("drivecard.internal_with_root"):
		return "phone"
	case strings.Contains(bus, "sd") && info.Removable, strings.Contains(bus, "mmc"):
		return "sdcard"
	case info.Kind == disk.KindUSB || strings.Contains(bus, "usb"):
		return "usb"
	}
	return "drive"
}

// DriveCard draws one drive in the list; s is its press state.
func DriveCard(info disk.Info, selected bool, s giode.PressState) giode.Element {
	border, bg := "theme.border", "theme.surface"
	switch {
	case selected:
		border, bg = "theme.primary", theme.Tint()
	case s.Hovered:
		border, bg = giode.Mix("theme.text", "theme.border", 0.18), theme.Raised()
	}
	well := giode.Mix("theme.text", "theme.surface", 0.06)
	iconColor := "theme.text"
	if selected {
		well, iconColor = giode.Mix("theme.primary", "theme.surface", 0.22), "theme.primary"
	}
	name := info.Model
	if name == "" || info.Kind == disk.KindVolume {
		// A partition by its own name, such as Local Disk (C:).
		name = info.Name
	}
	detail := []string{recovery.FormatBytes(info.Size)}
	if info.Bus != "" {
		detail = append(detail, info.Bus)
	}
	if info.Kind == disk.KindFiles {
		// Files, not sectors: the size is only known once they're
		// listed, and Model describes the source.
		name, detail = info.Name, []string{info.Model}
	}
	var tags []giode.Element
	for _, m := range info.Mounts {
		tags = append(tags, Tag(m, "theme.muted"))
	}
	if info.System {
		tags = append(tags, Tag(locales.T("drivecard.system_drive"), "theme.warning"))
	}
	if info.Removable {
		tags = append(tags, Tag(locales.T("drivecard.removable"), "theme.info"))
	}
	check := giode.Box(giode.Styles{Width: 20, Height: 20})
	if selected {
		check = giode.Box(giode.Styles{Width: 20, Height: 20, BorderRadius: 10, Background: "theme.primary",
			Align: giode.AlignCenter, Justify: giode.JustifyCenter}, icons.Icon("check", 13, "theme.onprimary"))
	}
	return giode.Box(giode.Styles{
		FlexDirection: giode.FlexDirectionColumn, Gap: theme.S3,
		Width: -1, Height: -1, Padding: giode.UniformInset(theme.S4),
		Background: bg, BorderWidth: 1, BorderColor: border, BorderRadius: theme.RadiusLarge,
		Cursor: giode.CursorPointer,
	},
		giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 44, Height: 44, BorderRadius: 12, Background: well, Align: giode.AlignCenter, Justify: giode.JustifyCenter},
				icons.Icon(DriveIcon(info), 22, iconColor)),
			giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
				giode.Text(name, theme.Heading().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
				giode.Text(strings.Join(detail, "  ·  "), theme.Secondary().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
			),
			check,
		),
		giode.Row(giode.Styles{Gap: 6}, tags...),
	)
}
