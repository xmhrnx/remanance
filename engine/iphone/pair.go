package iphone

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xmhrnx/remanance/engine/plist"
)

// pair makes sure this computer and the phone are paired, asking the
// phone's owner to trust the computer if they aren't, and reads the
// pairing into l. waiting is called while the phone shows the question.
func (l *lockdown) pair(ctx context.Context, waiting func()) error {
	rec, err := pairRecord(l.dev.udid)
	if err != nil {
		return err
	}
	if rec != nil {
		l.record = rec
		return nil
	}
	rec, err = newPairing(l)
	if err != nil {
		return err
	}
	pairing := map[string]any{}
	for _, k := range []string{"DeviceCertificate", "HostCertificate", "RootCertificate", "HostID", "SystemBUID"} {
		pairing[k] = rec[k]
	}
	for asked := false; ; {
		d, err := l.request("Pair", map[string]any{
			"PairRecord":      pairing,
			"ProtocolVersion": "2",
			"PairingOptions":  map[string]any{"ExtendedPairingErrors": true},
		})
		var le lockdownError
		if errors.As(err, &le) && le == "PairingDialogResponsePending" {
			if !asked && waiting != nil {
				waiting()
				asked = true
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
			continue
		}
		if err != nil {
			return err
		}
		if bag, ok := d["EscrowBag"].([]byte); ok {
			rec["EscrowBag"] = bag
		}
		break
	}
	if mac := plist.String(l.value("", "WiFiAddress")); mac != "" {
		rec["WiFiMACAddress"] = mac
	}
	if err := savePairRecord(l.dev, rec); err != nil {
		return fmt.Errorf("saving the pairing with the iPhone: %w", err)
	}
	l.record = rec
	return nil
}
