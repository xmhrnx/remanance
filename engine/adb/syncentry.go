package adb

import "time"

// entry is a file or folder as the sync service lists it.
type entry struct {
	name string
	mode uint32 // Unix
	size int64
	mod  time.Time
}
