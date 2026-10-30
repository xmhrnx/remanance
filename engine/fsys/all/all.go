// Package all registers every file system parser with package fsys.
package all

import (
	_ "github.com/xmhrnx/remanance/engine/fsys/exfat"
	_ "github.com/xmhrnx/remanance/engine/fsys/ext4"
	_ "github.com/xmhrnx/remanance/engine/fsys/fat"
	_ "github.com/xmhrnx/remanance/engine/fsys/ntfs"
)
