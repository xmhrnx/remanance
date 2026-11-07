package fat

import "encoding/binary"

// parseDir returns the entries of a directory's raw bytes, deleted ones
// included. base is the volume offset of raw[0]. Entries a deleted name's
// LFN can't vouch for keep their 8.3 name with "_" for the lost first
// letter.
func parseDir(raw []byte, base int64, includeDeleted bool) (out []dirent, valid int) {
	var lfn []lfnPart
	for off := 0; off+32 <= len(raw); off += 32 {
		e := raw[off : off+32]
		first := e[0]
		if first == 0 {
			// End of the directory. Anything after is left from before
			// and isn't trustworthy enough to list.
			break
		}
		deleted := first == entryDeleted
		if e[11] == attrLFN {
			if deleted && !includeDeleted {
				lfn = nil
				continue
			}
			p := parseLFN(e)
			if !deleted && first&0x40 != 0 {
				lfn = lfn[:0]
			}
			lfn = append(lfn, p)
			continue
		}
		parts := lfn
		lfn = nil
		if first == '.' || e[11]&attrVolumeID != 0 {
			continue
		}
		if deleted && !includeDeleted {
			continue
		}
		if !plausible(e, deleted) {
			continue
		}
		valid++
		d := dirent{
			deleted:  deleted,
			isDir:    e[11]&attrDirectory != 0,
			cluster:  uint32(binary.LittleEndian.Uint16(e[20:]))<<16 | uint32(binary.LittleEndian.Uint16(e[26:])),
			size:     int64(binary.LittleEndian.Uint32(e[28:])),
			created:  dosTime(binary.LittleEndian.Uint16(e[16:]), binary.LittleEndian.Uint16(e[14:])),
			modified: dosTime(binary.LittleEndian.Uint16(e[24:]), binary.LittleEndian.Uint16(e[22:])),
			accessed: dosTime(binary.LittleEndian.Uint16(e[18:]), 0),
			offset:   base + int64(off),
		}
		short := append([]byte(nil), e[:11]...)
		if first == 0x05 {
			short[0] = entryDeleted // 0x05 stands for a real 0xE5 first byte
		}
		long := ""
		if len(parts) > 0 {
			if deleted {
				// The short name's first letter is gone. Windows derives
				// it from the long name, so that letter (or "_" for a
				// non-ASCII one) must make the checksum match. Trying
				// every byte would match stale long names by chance.
				candidate := assembleLFN(parts)
				for _, c := range shortNameFirst(candidate) {
					short[0] = c
					if lfnChecksum(short) == parts[0].checksum {
						long = candidate
						break
					}
				}
				if long == "" {
					short[0] = '_'
				}
			} else if lfnChecksum(short) == parts[0].checksum {
				long = assembleLFN(parts)
			}
		} else if deleted {
			short[0] = '_'
		}
		if long != "" {
			d.name = long
		} else {
			tmp := append(append([]byte(nil), short...), e[11:]...)
			d.name = shortName(tmp)
		}
		if d.name == "" {
			continue
		}
		out = append(out, d)
	}
	return out, valid
}
