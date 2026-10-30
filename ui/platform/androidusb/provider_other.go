//go:build !android

package androidusb

// Register does nothing off Android: desktops reach USB drives through
// the operating system.
func Register() {}
