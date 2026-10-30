package androidusb

import (
	"strings"

	"github.com/mehran9675/giode/android"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/disk/usbmass"
)


// Register adds the USB drives to disk.List.
func Register() { disk.Register(provider{}) }

type provider struct{}

func (provider) List() ([]disk.Info, error) {
	list, err := android.CallString(bridgeClass, "list", "(Landroid/content/Context;)Ljava/lang/String;", android.Context())
	if err != nil {
		return nil, err
	}
	return parse(list), nil
}

func (provider) Handles(id string) bool { return strings.HasPrefix(id, prefix) }

func (provider) Open(info disk.Info) (disk.Device, error) {
	name := strings.TrimPrefix(info.ID, prefix)
	answer, err := android.CallString(bridgeClass, "open", "(Landroid/content/Context;Ljava/lang/String;)Ljava/lang/String;",
		android.Context(), name)
	if err != nil {
		return nil, err
	}
	fd, iface, in, out, err := parseOpen(answer)
	if err != nil {
		return nil, err
	}
	c := closer(name)
	dev, err := usbmass.Open(usbmass.NewDevFS(fd, iface, in, out), info, c)
	if err != nil {
		c.Close()
		return nil, err
	}
	return dev, nil
}

// closer releases the drive: Android closes the descriptor with the
// connection.
type closer string

func (c closer) Close() error {
	return android.CallVoid(bridgeClass, "close", "(Ljava/lang/String;)V", string(c))
}
