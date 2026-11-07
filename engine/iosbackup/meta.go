package iosbackup

import (
	"time"

	"github.com/xmhrnx/remanance/engine/fsys"
	"github.com/xmhrnx/remanance/engine/plist"
)

// meta fills in an entry's dates from the MBFile archived for it, and
// returns its size and key.
func meta(e *fsys.Entry, blob []byte) fileMeta {
	m := fileMeta{size: -1}
	v, err := plist.Parse(blob)
	if err != nil {
		return m
	}
	a := plist.Dict(v)
	objs, _ := a["$objects"].([]any)
	root := plist.Int(plist.Dict(a["$top"])["root"])
	if root <= 0 || root >= int64(len(objs)) {
		return m
	}
	f := plist.Dict(objs[root])
	if t := plist.Int(f["LastModified"]); t > 0 {
		e.Modified = time.Unix(t, 0)
	}
	if t := plist.Int(f["Birth"]); t > 0 {
		e.Created = time.Unix(t, 0)
	}
	if n, ok := f["Size"]; ok {
		m.size = plist.Int(n)
	}
	if ref, ok := f["EncryptionKey"].(plist.UID); ok && int(ref) < len(objs) {
		m.key, _ = plist.Dict(objs[ref])["NS.data"].([]byte)
	}
	return m
}
