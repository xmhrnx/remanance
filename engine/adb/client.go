package adb

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ErrNoADB means Android's platform tools aren't installed.
var ErrNoADB = errors.New("to scan an Android phone from this computer, install Android's platform tools (adb), " +
	"and turn on USB debugging on the phone, in Developer options")

// Device is a phone the adb server sees.
type Device struct {
	Serial string
	State  string // "device" when it can be used; "unauthorized" until the phone allows this computer
	Model  string
}

// Devices lists the phones connected.
func Devices() ([]Device, error) {
	out, err := query("host:devices-l")
	if err != nil {
		return nil, err
	}
	return parseDevices(out), nil
}

func parseDevices(out string) []Device {
	var ds []Device
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		d := Device{Serial: f[0], State: f[1], Model: "Android phone"}
		for _, kv := range f[2:] {
			if m, ok := strings.CutPrefix(kv, "model:"); ok {
				d.Model = strings.ReplaceAll(m, "_", " ")
			}
		}
		ds = append(ds, d)
	}
	return ds
}

// features lists what the phone's adb supports, such as ls_v2.
func features(serial string) string {
	out, _ := query("host-serial:" + serial + ":features")
	return out
}

// quote quotes a word for the phone's shell.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// status reads OKAY, or FAIL and why.
func status(c io.Reader) error {
	s := make([]byte, 4)
	if _, err := io.ReadFull(c, s); err != nil {
		return err
	}
	switch string(s) {
	case "OKAY":
		return nil
	case "FAIL":
		msg, err := readHex(c)
		if err != nil {
			return err
		}
		return fmt.Errorf("adb: %s", msg)
	}
	return fmt.Errorf("adb answered %q", s)
}

// readHex reads a reply after its length in four hex digits.
func readHex(c io.Reader) (string, error) {
	h := make([]byte, 4)
	if _, err := io.ReadFull(c, h); err != nil {
		return "", err
	}
	n, err := strconv.ParseUint(string(h), 16, 32)
	if err != nil {
		return "", err
	}
	b := make([]byte, n)
	_, err = io.ReadFull(c, b)
	return string(b), err
}
