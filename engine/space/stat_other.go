//go:build !unix

package space

import "io/fs"

// deviceOf is 0 where Go doesn't tell a file's drive; on Windows a
// drive mounted in a folder is a reparse point, which ReadDir reports
// as a link and the walk skips.
func deviceOf(fs.FileInfo) uint64 { return 0 }

// allocated is a file's size.
func allocated(info fs.FileInfo) int64 { return info.Size() }
