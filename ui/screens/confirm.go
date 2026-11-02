package screens

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/xmhrnx/remanance/locales"
	"github.com/xmhrnx/remanance/ui/components"
	"github.com/xmhrnx/remanance/ui/icons"
	"github.com/xmhrnx/remanance/ui/theme"
)

// Confirm asks before something that can't easily be taken back.
type Confirm struct {
	open           bool
	dlg            *dialog.Dialog
	title, message string
	yes, no        *giode.ButtonElement
	action         func()
}

// NewConfirm builds the dialog, closed.
func NewConfirm() *Confirm {
	c := &Confirm{}
	c.dlg = giode.Dialog(&c.open, c.content, giode.Styles{Width: 440, BorderRadius: theme.RadiusLarge, Padding: giode.UniformInset(theme.S6),
		Background: "theme.surface", BorderWidth: 1, BorderColor: "theme.border"})
	c.yes = components.PrimaryButton("", "trash").OnClick(func() {
		c.open = false
		if c.action != nil {
			c.action()
		}
	})
	c.no = components.GhostButton(locales.T("confirm.cancel"), "").OnClick(func() { c.open = false })
	return c
}

// Ask shows the dialog; action runs if the user agrees.
func (c *Confirm) Ask(title, message, yes string, action func()) {
	c.title, c.message, c.action = title, message, action
	c.yes.Label(yes)
	c.open = true
}

// View places the dialog; it takes no space while closed.
func (c *Confirm) View() giode.Element { return c.dlg }

func (c *Confirm) content() giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Gap: theme.S4},
		giode.Row(giode.Styles{Gap: theme.S3, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 40, Height: 40, BorderRadius: 12, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
				Background: giode.Mix("theme.danger", "theme.surface", 0.14)}, icons.Icon("trash", 20, "theme.danger")),
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(c.title, theme.Heading())),
		),
		giode.Text(c.message, theme.Secondary()),
		giode.Row(giode.Styles{Width: -1, Gap: theme.S2, Justify: giode.JustifyEnd}, c.no, c.yes),
	)
}
