package model

import "github.com/xmhrnx/remanance/engine/recovery"

func bytes(n int64) string { return recovery.FormatBytes(n) }
