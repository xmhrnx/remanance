// Package androidusb lists the USB drives plugged into an Android phone
// with an OTG adapter, and opens them for a deep scan without root:
// UsbBridge.java gets the drive from Android's USB host API once the
// user allows it, and engine/disk/usbmass reads it. giode build compiles
// UsbBridge into the APK (android.java in giode.toml).
package androidusb

import (
	"strconv"
	"strings"

	"github.com/xmhrnx/remanance/engine/disk"
)

type openError struct{ msg string }

// prefix starts the ID of every drive found here.
const prefix = "usb:"

// bridgeClass is UsbBridge.java's class, as JNI names it.
const bridgeClass = "io/github/mehran9675/datadoc/UsbBridge"

// parse reads UsbBridge.list's lines into drives.
func parse(list string) []disk.Info {
	var out []disk.Info
	for _, line := range strings.Split(strings.TrimSpace(list), "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 6 {
			continue
		}
		name := strings.TrimSpace(f[3] + " " + f[4])
		if name == "" {
			vendor, _ := strconv.Atoi(f[1])
			product, _ := strconv.Atoi(f[2])
			name = "USB drive " + strconv.FormatInt(int64(vendor), 16) + ":" + strconv.FormatInt(int64(product), 16)
		}
		out = append(out, disk.Info{
			ID:        prefix + f[0],
			Path:      f[0],
			Name:      name,
			Model:     name,
			Bus:       "USB",
			Kind:      disk.KindUSB,
			Removable: true,
		})
	}
	return out
}

// parseOpen reads UsbBridge.open's answer: "fd iface in out".
func parseOpen(s string) (fd, iface, in, out int, err error) {
	if msg, ok := strings.CutPrefix(s, "error: "); ok {
		return 0, 0, 0, 0, &openError{msg}
	}
	var v [4]int
	f := strings.Fields(s)
	if len(f) != 4 {
		return 0, 0, 0, 0, &openError{"unexpected answer from Android: " + s}
	}
	for i := range v {
		if v[i], err = strconv.Atoi(f[i]); err != nil {
			return 0, 0, 0, 0, &openError{"unexpected answer from Android: " + s}
		}
	}
	return v[0], v[1], v[2], v[3], nil
}

func (e *openError) Error() string { return e.msg }
