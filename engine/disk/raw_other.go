//go:build !windows && !(linux && !android) && !(darwin && !ios)

package disk

// Phones and other platforms have no raw access to the OS's own drives:
// Android reaches USB drives through package usbms, iOS works on images.

func listPhysical() ([]Info, error) { return nil, nil }

func openPhysical(Info) (Device, error) { return nil, ErrNotSupported }
