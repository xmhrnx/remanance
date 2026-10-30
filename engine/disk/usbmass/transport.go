// Package usbmass reads USB drives directly, without the operating
// system's storage driver: SCSI commands over the USB Bulk-Only
// Transport, the protocol of every USB stick, card reader and external
// drive. On Android it reaches a drive plugged in with an OTG adapter
// without root, through the USB host API.
package usbmass

import (
	"errors"
	"time"
)

// Transport moves bytes over a drive's two bulk endpoints.
type Transport interface {
	// BulkOut sends data to the drive.
	BulkOut(data []byte, timeout time.Duration) (int, error)
	// BulkIn reads up to len(data) bytes from the drive.
	BulkIn(data []byte, timeout time.Duration) (int, error)
	// ClearHalt clears a stalled endpoint: the drive's way of refusing
	// a command or a transfer.
	ClearHalt(in bool) error
	// Reset recovers the drive after a protocol error: Bulk-Only Mass
	// Storage Reset, then clearing both endpoints.
	Reset() error
	// MaxTransfer is the most one BulkIn or BulkOut can move.
	MaxTransfer() int
}

// ErrStall is returned by a Transport when the endpoint stalled.
var ErrStall = errors.New("usb: endpoint stalled")
