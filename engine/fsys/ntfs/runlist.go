package ntfs

// run is a stretch of clusters in a run list. lcn is -1 for sparse runs.
type run struct {
	lcn    int64
	length int64 // clusters
}

// decodeRuns decodes an NTFS mapping pairs array. Each run is a header
// byte (low nibble: size of the length, high nibble: size of the offset),
// the length, and the offset relative to the previous run's start.
func decodeRuns(b []byte) []run {
	var out []run
	lcn := int64(0)
	for i := 0; i < len(b) && b[i] != 0; {
		lenSize := int(b[i] & 0x0F)
		offSize := int(b[i] >> 4)
		i++
		if lenSize == 0 || lenSize > 8 || offSize > 8 || i+lenSize+offSize > len(b) {
			break
		}
		var length int64
		for j := lenSize - 1; j >= 0; j-- {
			length = length<<8 | int64(b[i+j])
		}
		i += lenSize
		if offSize == 0 {
			out = append(out, run{lcn: -1, length: length})
			continue
		}
		var delta int64
		for j := offSize - 1; j >= 0; j-- {
			delta = delta<<8 | int64(b[i+j])
		}
		// Sign-extend the offset.
		if b[i+offSize-1]&0x80 != 0 {
			delta -= 1 << (8 * uint(offSize))
		}
		i += offSize
		lcn += delta
		if length <= 0 || lcn < 0 {
			break
		}
		out = append(out, run{lcn: lcn, length: length})
	}
	return out
}
