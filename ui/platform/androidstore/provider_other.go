//go:build !android

package androidstore

// Register does nothing off Android.
func Register() {}

// EnsureAccess is nil off Android, where files are read as the user.
func EnsureAccess() error { return nil }
