//go:build android || ios

package iosbackups

// Register does nothing on a phone, which keeps no backups of others.
func Register() {}
