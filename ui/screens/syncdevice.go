package screens

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/input"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/model"
	"github.com/xmhrnx/remanance/ui/theme"
)

// AddDevice adds one of the user's devices: one found nearby, or one by
// the ID it shows.
type AddDevice struct {
	m      *model.Model
	open   bool
	dlg    *dialog.Dialog
	id     *input.Input
	addr   *input.Input
	add    *giode.ButtonElement
	cancel *giode.ButtonElement
	err    string
}

// NewAddDevice builds the dialog, closed.
func NewAddDevice(m *model.Model) *AddDevice {
	d := &AddDevice{m: m}
	d.dlg = giode.Dialog(&d.open, d.content, giode.Styles{Width: 520, BorderRadius: theme.RadiusLarge, Padding: giode.UniformInset(theme.S6),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"})
	d.id = giode.Input.Text("XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX", giode.Styles{FontSize: 13, FontFamily: theme.Mono, Padding: giode.SymmetricInset(10, 9), BorderRadius: theme.Radius}).
		Submit(func(string) { d.addTyped() })
	d.addr = giode.Input.Text("Its address, if it's not on this network: 203.0.113.5:22100", giode.Styles{FontSize: 13, Padding: giode.SymmetricInset(10, 9), BorderRadius: theme.Radius})
	d.add = components.PrimaryButton("Add device", "plus").OnClick(d.addTyped)
	d.cancel = components.GhostButton("Cancel", "").OnClick(func() { d.open = false })
	return d
}

// Open shows the dialog.
func (d *AddDevice) Open() {
	d.err = ""
	d.id.SetText("")
	d.addr.SetText("")
	d.open = true
}

func (d *AddDevice) addTyped() {
	d.addID(d.id.Text(), "", d.addr.Text())
}

func (d *AddDevice) addID(id, name, addr string) {
	if d.m.Sync.Node == nil {
		return
	}
	if err := d.m.Sync.Node.AddDevice(id, name, addr); err != nil {
		d.err = err.Error()
		return
	}
	d.open = false
	d.m.RefreshSync()
}

// View places the dialog; it takes no space while closed.
func (d *AddDevice) View() giode.Element { return d.dlg }

func (d *AddDevice) content() giode.Element {
	kids := []giode.Element{
		giode.Row(giode.Styles{Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 40, Height: 40, BorderRadius: 12, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
				Background: giode.Mix("theme.primary", "theme.surface", 0.14)}, icons.Icon("devices", 20, "theme.primary")),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
				giode.Text("Add a device", theme.Heading()),
				giode.Text("Open Remanence's Sync on the other device; it lists this one, or shows its ID to type here.", theme.Caption())),
		),
	}
	if near := d.m.Sync.Nearby; len(near) > 0 {
		var rows []giode.Element
		for _, nb := range near {
			nb := nb
			p := giode.Keyed("nearby", nb.ID, func() *giode.PressableElement {
				return giode.Pressable(func(giode.PressState) giode.Element { return nil })
			})
			rows = append(rows, p.Content(func(s giode.PressState) giode.Element {
				bg := theme.Raised()
				if s.Hovered {
					bg = giode.Mix("theme.primary", "theme.surface", 0.12)
				}
				return giode.Row(giode.Styles{Width: -1, Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(theme.S3, 10),
					Background: bg, BorderRadius: theme.Radius},
					icons.Icon("devices", 18, "theme.primary"),
					giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
						giode.Text(nb.Name, theme.Body().With(giode.Styles{FontSize: 13.5, FontWeight: giode.FontWeightSemiBold})),
						giode.Text(nb.ID, theme.Code().With(giode.Styles{FontSize: 10.5, Color: "theme.muted"}))),
					giode.Text("Add", theme.Body().With(giode.Styles{FontSize: 13, Color: "theme.primary", FontWeight: giode.FontWeightSemiBold})),
				)
			}).OnClick(func(giode.ClickEvent) { d.addID(nb.ID, nb.Name, "") }).Label("Add "+nb.Name))
		}
		kids = append(kids, section("On this network", rows...))
	}
	kids = append(kids, section("By its ID", d.id, d.addr))
	if d.err != "" {
		kids = append(kids, giode.Text(d.err, theme.Caption().With(giode.Styles{Color: "theme.danger"})))
	}
	kids = append(kids, giode.Text("The other device then asks its owner to accept this one. Devices talk only to each other, encrypted; nothing goes through a server.", theme.Caption()),
		giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Justify: giode.JustifyEnd}, d.cancel, d.add))
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S4}, kids...)
}
