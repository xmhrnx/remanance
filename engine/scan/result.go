package scan

import (
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/partition"
)

// VolumeResult is one volume the scan read.
type VolumeResult struct {
	Partition partition.Partition
	Volume    *fsys.Volume // nil when the file system couldn't be read
	Err       error
}

// Result is everything a scan found.
type Result struct {
	Device  disk.Info
	Mode    Mode
	Table   partition.Table
	Volumes []*VolumeResult
	// Carved holds the files found by signature, in folders by category
	// and type. nil after a quick scan.
	Carved   *fsys.Volume
	Bad      []disk.Range
	Started  time.Time
	Duration time.Duration
	Map      []Cell
}

// Stats totals what was found across every volume.
func (r *Result) Stats() (files, deleted, carved int) {
	for _, v := range r.Volumes {
		if v.Volume == nil {
			continue
		}
		s := v.Volume.Stats()
		files += s.Files
		deleted += s.Deleted
	}
	if r.Carved != nil {
		carved = r.Carved.Stats().Files
	}
	return
}
