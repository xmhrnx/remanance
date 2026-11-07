package fsys

import (
	"fmt"
	"io"
	"sync"
)

// Codec decodes data stored in fixed-size units (NTFS compression).
// It reads unit number n of an entry from dev and returns its plain bytes.
type Codec func(dev io.ReaderAt, e *Entry, n int64) ([]byte, error)

var (
	codecsMu sync.RWMutex
	codecs   = map[string]Codec{}
)

// RegisterCodec makes a codec available by name.
func RegisterCodec(name string, c Codec) {
	codecsMu.Lock()
	defer codecsMu.Unlock()
	codecs[name] = c
}

// Open returns a reader for an entry's data on dev. Reads past Size return
// io.EOF.
func Open(dev io.ReaderAt, e *Entry) (io.ReaderAt, error) {
	if e.Codec != "" {
		codecsMu.RLock()
		c := codecs[e.Codec]
		codecsMu.RUnlock()
		if c == nil {
			return nil, fmt.Errorf("fsys: no decoder for %q", e.Codec)
		}
		return &unitReader{dev: dev, e: e, codec: c}, nil
	}
	if len(e.Inline) > 0 || len(e.Runs) == 0 {
		return &inlineReader{data: e.Inline, size: e.Size}, nil
	}
	return &runReader{dev: dev, runs: e.Runs, size: e.Size}, nil
}
