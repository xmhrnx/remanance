package ext4

import (
	"bytes"
	"encoding/binary"
	"unicode/utf8"
)

// dirEntry is a name in a directory block. deleted entries were found in
// the slack space a removal leaves behind.
type dirEntry struct {
	inode    uint32
	name     string
	fileType byte
	deleted  bool
}

const (
	ftFile    = 1
	ftDir     = 2
	ftSymlink = 7
)

func validName(n []byte) bool {
	if len(n) == 0 || bytes.IndexByte(n, 0) >= 0 || bytes.IndexByte(n, '/') >= 0 {
		return false
	}
	return utf8.Valid(n)
}

// parseDirBlock lists a directory block's live entries, and the deleted
// ones whose bytes survive. Removing an entry only grows the record before
// it to cover it, so the old entry stays readable inside that record's
// slack.
func parseDirBlock(b []byte, maxInode uint32, filetype bool) []dirEntry {
	le := binary.LittleEndian
	var out []dirEntry
	for off := 0; off+8 <= len(b); {
		ino := le.Uint32(b[off:])
		recLen := int(le.Uint16(b[off+4:]))
		nameLen := int(b[off+6])
		ft := b[off+7]
		if !filetype {
			nameLen |= int(ft) << 8
			ft = 0
		}
		if recLen < 8 || off+recLen > len(b) || recLen%4 != 0 {
			break
		}
		used := 8 + (nameLen+3)&^3
		if ino != 0 && nameLen > 0 && off+8+nameLen <= len(b) && ino <= maxInode {
			name := b[off+8 : off+8+nameLen]
			if validName(name) {
				out = append(out, dirEntry{inode: ino, name: string(name), fileType: ft})
			}
		}
		if nameLen == 0 || ino == 0 {
			used = 8
			if ino == 0 && nameLen > 0 {
				used = 8 + (nameLen+3)&^3
			}
		}
		// Look for entries in the slack after this one.
		for s := off + used; s+8 <= off+recLen; s += 4 {
			if d, ok := slackEntry(b[s:off+recLen], maxInode, filetype); ok {
				out = append(out, d)
				s += 8 + (len(d.name)+3)&^3 - 4
			}
		}
		off += recLen
	}
	return out
}

func slackEntry(b []byte, maxInode uint32, filetype bool) (dirEntry, bool) {
	le := binary.LittleEndian
	ino := le.Uint32(b)
	recLen := int(le.Uint16(b[4:]))
	nameLen := int(b[6])
	ft := b[7]
	if ino == 0 || ino > maxInode || nameLen == 0 || 8+nameLen > len(b) || recLen < 8+nameLen || recLen%4 != 0 {
		return dirEntry{}, false
	}
	if filetype && (ft == 0 || ft > 7) {
		return dirEntry{}, false
	}
	name := b[8 : 8+nameLen]
	if !validName(name) || string(name) == "." || string(name) == ".." {
		return dirEntry{}, false
	}
	return dirEntry{inode: ino, name: string(name), fileType: ft, deleted: true}, true
}
