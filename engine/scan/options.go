// Package scan runs a recovery scan of a device: it reads the partition
// table and every file system, and in a deep scan also reads the whole
// device for lost partitions and files known only by their content.
package scan

import "github.com/mehran9675/remanence/engine/carve"

// Mode is how thorough a scan is.
type Mode int

const (
	// Quick reads partition tables and file system metadata: deleted
	// files the file system still describes. Seconds to minutes.
	Quick Mode = iota
	// Deep also reads every sector: lost and reformatted partitions,
	// orphaned folders, and files found by signature. As long as reading
	// the whole device takes.
	Deep
)

// Options configure a scan.
type Options struct {
	Mode Mode
	// Signatures to carve for in a deep scan; nil means all of them,
	// empty (non-nil) means no carving.
	Signatures []*carve.Signature
	// Start and End limit the deep scan's sector search to part of the
	// device (End 0 means the end).
	Start, End int64
}
