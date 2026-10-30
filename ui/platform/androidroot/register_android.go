//go:build android

package androidroot

import "github.com/xmhrnx/remanance/engine/disk"

// Register adds the phone's own storage to disk.List on a rooted phone.
func Register() {
	if hasSU() {
		disk.Register(&provider{start: func() (*session, error) { return start("su") }})
	}
}
