package fsys

// Bitmap is an allocation bitmap: bit n set means cluster n is in use.
type Bitmap []byte

// Used reports whether cluster n is allocated. Clusters past the bitmap
// count as used, the safe answer for a health estimate.
func (b Bitmap) Used(n int64) bool {
	if n < 0 || n/8 >= int64(len(b)) {
		return true
	}
	return b[n/8]&(1<<(n%8)) != 0
}

// Health is the share, 0 to 100, of the clusters under runs that are
// free now. Runs are device offsets; base is the device offset of
// cluster 0. A deleted file whose clusters were all reallocated scores 0.
func Health(runs []Run, clusterSize, base int64, used func(cluster int64) bool) uint8 {
	var total, free int64
	for _, r := range runs {
		if r.Offset < 0 {
			continue
		}
		first := (r.Offset - base) / clusterSize
		n := (r.Length + clusterSize - 1) / clusterSize
		// Sample long runs: a 4 GB video has a million clusters.
		step := int64(1)
		if n > 4096 {
			step = n / 4096
		}
		for c := int64(0); c < n; c += step {
			total++
			if !used(first + c) {
				free++
			}
		}
	}
	if total == 0 {
		return 100
	}
	return uint8(free * 100 / total)
}
