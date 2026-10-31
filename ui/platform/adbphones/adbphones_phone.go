//go:build android || ios

package adbphones

// Register does nothing on a phone, which can't be an ADB host here.
func Register() {}
