// Package all registers every file system parser with package fsys.
package all

import (
	_ "github.com/mehran9675/remanence/engine/fsys/exfat"
	_ "github.com/mehran9675/remanence/engine/fsys/ext4"
	_ "github.com/mehran9675/remanence/engine/fsys/fat"
	_ "github.com/mehran9675/remanence/engine/fsys/ntfs"
)
