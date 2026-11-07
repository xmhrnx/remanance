package ntfs

import "github.com/xmhrnx/remanance/engine/fsys"

func (vol *volume) loadBitmap(nodes []*node) {
	if recordBitmap >= len(nodes) || nodes[recordBitmap] == nil {
		vol.v.Warn("$Bitmap is missing; recovery chances can't be estimated")
		return
	}
	n := nodes[recordBitmap]
	if len(n.data) == 0 {
		return
	}
	size := n.data[0].dataSize
	for _, p := range n.data {
		if p.startVCN == 0 {
			size = p.dataSize
		}
	}
	if n.data[0].resident {
		vol.bitmap = fsys.Bitmap(n.data[0].value)
		return
	}
	vol.bitmap = fsys.Bitmap(vol.readRuns(joinRuns(n.data), size))
}

func (vol *volume) label(rec *record) {
	for _, a := range rec.attrs {
		if a.typ == 0x60 && a.resident { // $VOLUME_NAME
			vol.v.Label = utf16String(a.value)
		}
	}
}
