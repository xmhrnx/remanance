//go:build android || ios

package iphones

// Register does nothing on a phone, which can't host another by USB.
func Register() {}
