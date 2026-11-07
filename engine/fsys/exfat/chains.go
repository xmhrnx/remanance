package exfat

import (
	"encoding/binary"

	"github.com/xmhrnx/remanance/engine/fsys"
)

func (vol *volume) next(c uint32) uint32 {
	i := int64(c) * 4
	if i+4 > int64(len(vol.fat)) {
		return 0
	}
	return binary.LittleEndian.Uint32(vol.fat[i:])
}

func (vol *volume) used(c uint32) bool {
	if vol.bitmap == nil {
		return true
	}
	return vol.bitmap.Used(int64(c) - 2)
}

// chain returns up to n clusters of a FAT chain from first.
func (vol *volume) chain(first uint32, n int64) []uint32 {
	var out []uint32
	seen := map[uint32]bool{}
	for c := first; vol.b.valid(c) && int64(len(out)) < n && !seen[c]; c = vol.next(c) {
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func (vol *volume) readClusters(cl []uint32, size int64) []byte {
	cs := vol.b.clusterSize
	out := make([]byte, int64(len(cl))*cs)
	for i, c := range cl {
		vol.r.ReadAt(out[int64(i)*cs:int64(i+1)*cs], vol.b.clusterOffset(c))
	}
	if size >= 0 && size < int64(len(out)) {
		out = out[:size]
	}
	return out
}

func (vol *volume) readChain(first uint32, size int64) []byte {
	cs := vol.b.clusterSize
	return vol.readClusters(vol.chain(first, (size+cs-1)/cs), size)
}

// clustersOf places a file's data: contiguous when the stream says so,
// else by its FAT chain. Deleting a file clears its bitmap bits but
// leaves the chain, unless the chain no longer adds up.
func (vol *volume) clustersOf(f fileSet) (cl []uint32, guessed bool) {
	cs := vol.b.clusterSize
	need := (f.size + cs - 1) / cs
	if !f.noChain {
		cl = vol.chain(f.cluster, need)
		if int64(len(cl)) == need {
			return cl, false
		}
	}
	cl = cl[:0]
	for c := f.cluster; int64(len(cl)) < need && vol.b.valid(c); c++ {
		cl = append(cl, c)
	}
	return cl, !f.noChain
}

func (vol *volume) runs(cl []uint32) []fsys.Run {
	cs := vol.b.clusterSize
	var out []fsys.Run
	for i := 0; i < len(cl); {
		j := i + 1
		for j < len(cl) && cl[j] == cl[j-1]+1 {
			j++
		}
		out = append(out, fsys.Run{Offset: vol.devOff + vol.b.clusterOffset(cl[i]), Length: int64(j-i) * cs})
		i = j
	}
	return out
}
