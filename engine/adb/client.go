// Package adb reads an Android phone connected to this computer by USB,
// through Android's debug bridge: its shared storage, as the phone's
// own app reads it, and, on a rooted phone, the databases apps keep
// texts, calls and chats in (see engine/appdata). It talks to the adb
// server Android's platform tools run, starting it if it can.
package adb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ErrNoADB means Android's platform tools aren't installed.
var ErrNoADB = errors.New("to scan an Android phone from this computer, install Android's platform tools (adb), " +
	"and turn on USB debugging on the phone, in Developer options")

// addr is where the adb server listens.
func addr() string {
	port := os.Getenv("ANDROID_ADB_SERVER_PORT")
	if port == "" {
		port = "5037"
	}
	return "127.0.0.1:" + port
}

// adbPath finds the adb program.
func adbPath() string {
	if p, err := exec.LookPath("adb"); err == nil {
		return p
	}
	exe := "adb"
	if runtime.GOOS == "windows" {
		exe = "adb.exe"
	}
	home, _ := os.UserHomeDir()
	var dirs []string
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if d := os.Getenv(env); d != "" {
			dirs = append(dirs, filepath.Join(d, "platform-tools"))
		}
	}
	switch runtime.GOOS {
	case "windows":
		dirs = append(dirs, filepath.Join(os.Getenv("LOCALAPPDATA"), "Android", "Sdk", "platform-tools"))
	case "darwin":
		dirs = append(dirs, filepath.Join(home, "Library", "Android", "sdk", "platform-tools"), "/opt/homebrew/bin")
	default:
		dirs = append(dirs, filepath.Join(home, "Android", "Sdk", "platform-tools"))
	}
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(d, exe)); err == nil {
			return filepath.Join(d, exe)
		}
	}
	return ""
}

// dial connects to the adb server, starting it if it isn't running.
func dial() (net.Conn, error) {
	c, err := net.DialTimeout("tcp", addr(), 2*time.Second)
	if err == nil {
		return c, nil
	}
	adb := adbPath()
	if adb == "" {
		return nil, ErrNoADB
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, adb, "start-server")
	hide(cmd)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("starting adb: %w", err)
	}
	return net.DialTimeout("tcp", addr(), 2*time.Second)
}

// send sends a request to the server, its length first in four hex
// digits, and reads whether it was taken.
func send(c net.Conn, req string) error {
	if _, err := fmt.Fprintf(c, "%04x%s", len(req), req); err != nil {
		return err
	}
	return status(c)
}

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

// query sends a request to the server itself and reads its reply.
func query(req string) (string, error) {
	c, err := dial()
	if err != nil {
		return "", err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(10 * time.Second))
	if err := send(c, req); err != nil {
		return "", err
	}
	return readHex(c)
}

// open connects to a service on the phone with the given serial number.
func open(serial, service string) (net.Conn, error) {
	c, err := dial()
	if err != nil {
		return nil, err
	}
	if err := send(c, "host:transport:"+serial); err != nil {
		c.Close()
		return nil, err
	}
	if err := send(c, service); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// run runs a command on the phone, its output not mangled by a
// terminal, and returns what it printed.
func run(serial, command string, timeout time.Duration) ([]byte, error) {
	c, err := open(serial, "exec:"+command)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(timeout))
	return io.ReadAll(c)
}

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
