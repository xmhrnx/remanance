package iphone

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xmhrnx/remanance/engine/plist"
)

// Phone is an iPhone or iPad connected by USB.
type Phone struct {
	UDID    string
	Name    string // "Sam's iPhone"
	Product string // "iPhone15,2"
	IOS     string
	dev     device
}

// Connected lists the iPhones and iPads connected by USB.
func Connected() ([]Phone, error) {
	devs, err := devices()
	if err != nil {
		return nil, err
	}
	var out []Phone
	for _, d := range devs {
		p := Phone{UDID: d.udid, Name: "iPhone", dev: d}
		if l, err := openLockdown(d); err == nil {
			if n := plist.String(l.value("", "DeviceName")); n != "" {
				p.Name = n
			}
			p.Product = plist.String(l.value("", "ProductType"))
			p.IOS = plist.String(l.value("", "ProductVersion"))
			l.Close()
		}
		out = append(out, p)
	}
	return out, nil
}

// Progress says how a backup is going.
type Progress struct {
	Step     string
	Fraction float64 // 0 to 1; 0 while it isn't known
	Bytes    int64   // received so far
}

// Steps of a backup.
const (
	StepTrust   = "Tap Trust on the iPhone, and enter its passcode"
	StepStart   = "Starting the backup"
	StepBackup  = "Backing up the iPhone"
	StepUnlock  = "Unlock the iPhone to continue"
	StepWriting = "Finishing the backup"
)

// Backup backs the phone up into dir, in a folder named after it, the
// way Finder lays a backup out, and returns that folder. An earlier
// backup there makes it quicker: the phone sends only what changed.
// If the phone encrypts its backups, this one is encrypted too, with the
// password set on the phone.
func Backup(ctx context.Context, p Phone, dir string, progress func(Progress)) (string, error) {
	report := func(pr Progress) {
		if progress != nil {
			progress(pr)
		}
	}
	report(Progress{Step: StepStart})
	l, err := openLockdown(p.dev)
	if err != nil {
		return "", err
	}
	defer l.Close()
	stop := context.AfterFunc(ctx, func() { l.Close() })
	defer stop()
	if err := l.pair(ctx, func() { report(Progress{Step: StepTrust}) }); err != nil {
		return "", wrap(ctx, err)
	}
	if err := l.startSession(); err != nil {
		return "", wrap(ctx, err)
	}
	values := map[string]any{
		"Device Name":       l.value("", "DeviceName"),
		"Display Name":      l.value("", "DeviceName"),
		"Product Type":      l.value("", "ProductType"),
		"Product Version":   l.value("", "ProductVersion"),
		"Build Version":     l.value("", "BuildVersion"),
		"Serial Number":     l.value("", "SerialNumber"),
		"Unique Identifier": p.UDID,
		"Target Identifier": p.UDID,
		"Target Type":       "Device",
	}

	svc, err := l.startService("com.apple.mobilebackup2", true)
	if err != nil {
		return "", wrap(ctx, err)
	}
	defer svc.Close()
	stopSvc := context.AfterFunc(ctx, func() { svc.Close() })
	defer stopSvc()
	if err := handshake(svc); err != nil {
		return "", wrap(ctx, err)
	}

	out := filepath.Join(dir, p.UDID)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	info, err := infoPlist(values)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(out, "Info.plist"), info, 0o644); err != nil {
		return "", err
	}
	if err := svc.sendBinary([]any{"DLMessageProcessMessage", map[string]any{
		"MessageName":      "Backup",
		"TargetIdentifier": p.UDID,
	}}); err != nil {
		return "", wrap(ctx, err)
	}

	lk := &link{c: svc, root: dir}
	var got int64
	fraction := 0.0
	report(Progress{Step: StepBackup})
	for {
		v, err := svc.recv()
		if err != nil {
			return "", wrap(ctx, err)
		}
		msg, _ := v.([]any)
		if len(msg) == 0 {
			return "", fmt.Errorf("the iPhone sent %v during the backup", v)
		}
		if f, ok := progressOf(msg); ok {
			fraction = f / 100
		}
		report(Progress{Step: StepBackup, Fraction: fraction, Bytes: got})
		switch name := plist.String(msg[0]); name {
		case "DLMessageDownloadFiles":
			list, _ := arg(msg, 1).([]any)
			err = lk.sendFiles(list)
		case "DLMessageUploadFiles":
			err = lk.receiveFiles(func(n int64) {
				got += n
				report(Progress{Step: StepBackup, Fraction: fraction, Bytes: got})
			})
		case "DLMessageGetFreeDiskSpace":
			err = lk.status(0, "", freeSpace(dir))
		case "DLContentsOfDirectory":
			err = lk.contents(plist.String(arg(msg, 1)))
		case "DLMessageCreateDirectory":
			err = lk.mkdir(plist.String(arg(msg, 1)))
		case "DLMessageMoveFiles", "DLMessageMoveItems":
			err = lk.move(plist.Dict(arg(msg, 1)))
		case "DLMessageRemoveFiles", "DLMessageRemoveItems":
			list, _ := arg(msg, 1).([]any)
			err = lk.remove(list)
		case "DLMessageCopyItem":
			err = lk.copyItem(plist.String(arg(msg, 1)), plist.String(arg(msg, 2)))
		case "DLMessageDisconnect":
			return "", fmt.Errorf("the iPhone ended the backup")
		case "DLMessageProcessMessage":
			// The result.
			d := plist.Dict(arg(msg, 1))
			if code := plist.Int(d["ErrorCode"]); code != 0 {
				return "", backupError(code, plist.String(d["ErrorDescription"]))
			}
			report(Progress{Step: StepWriting, Fraction: 1, Bytes: got})
			return out, nil
		default:
			err = lk.status(-1, "Operation not supported", nil)
		}
		if err != nil {
			return "", wrap(ctx, err)
		}
	}
}

func arg(msg []any, i int) any {
	if i < len(msg) {
		return msg[i]
	}
	return nil
}

// handshake agrees on DeviceLink's version, then the backup protocol's.
func handshake(c *conn) error {
	v, err := c.recv()
	if err != nil {
		return err
	}
	if m, _ := v.([]any); len(m) == 0 || m[0] != "DLMessageVersionExchange" {
		return fmt.Errorf("the backup service began with %v", v)
	}
	if err := c.sendBinary([]any{"DLMessageVersionExchange", "DLVersionsOk", int64(300)}); err != nil {
		return err
	}
	if v, err = c.recv(); err != nil {
		return err
	}
	if m, _ := v.([]any); len(m) == 0 || m[0] != "DLMessageDeviceReady" {
		return fmt.Errorf("the backup service isn't ready: %v", v)
	}
	if err := c.sendBinary([]any{"DLMessageProcessMessage", map[string]any{
		"MessageName":               "Hello",
		"SupportedProtocolVersions": []any{2.0, 2.1},
	}}); err != nil {
		return err
	}
	if v, err = c.recv(); err != nil {
		return err
	}
	m, _ := v.([]any)
	if len(m) < 2 {
		return fmt.Errorf("the backup service answered %v", v)
	}
	if code := plist.Int(plist.Dict(m[1])["ErrorCode"]); code != 0 {
		return fmt.Errorf("the backup service doesn't speak this version (error %d)", code)
	}
	return nil
}

// backupError explains the phone's reason for a failed backup.
func backupError(code int64, desc string) error {
	switch code {
	case 208:
		return ErrLocked
	case 105:
		return errors.New("there isn't enough free space here for the backup")
	}
	if desc == "" {
		desc = fmt.Sprintf("error %d", code)
	}
	return fmt.Errorf("the iPhone stopped the backup: %s", desc)
}

// wrap prefers the reason a cancelled backup stopped to the broken
// connection it left.
func wrap(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
