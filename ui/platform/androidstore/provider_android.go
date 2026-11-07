//go:build android

package androidstore

import (
	"strings"

	"github.com/mehran9675/giode/android"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/phonestore"
)

type provider struct{}

// Register adds the phone's storage to disk.List.
func Register() { disk.Register(provider{}) }

func (provider) List() ([]disk.Info, error) {
	return []disk.Info{phonestore.Info(phonestore.Root)}, nil
}

func (provider) Handles(id string) bool { return strings.HasPrefix(id, "phonestore:") }

func (provider) Open(disk.Info) (disk.Device, error) {
	ok, err := android.CallString(bridgeClass, "access", "(Landroid/content/Context;)Ljava/lang/String;", android.Context())
	if err != nil {
		return nil, err
	}
	if ok != "yes" {
		if err := android.CallVoid(bridgeClass, "request", "(Landroid/content/Context;)V", android.Context()); err != nil {
			return nil, err
		}
		return nil, ErrAccess
	}
	return phonestore.Open(phonestore.Root)
}

// EnsureAccess checks DataDoc may read every app's files, which the
// file tools need, and opens the settings to allow it if not.
func EnsureAccess() error {
	ok, err := android.CallString(bridgeClass, "access", "(Landroid/content/Context;)Ljava/lang/String;", android.Context())
	if err != nil || ok == "yes" {
		return err
	}
	if err := android.CallVoid(bridgeClass, "request", "(Landroid/content/Context;)V", android.Context()); err != nil {
		return err
	}
	return ErrFileAccess
}
