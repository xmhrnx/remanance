package disk

import (
	"sync"
	"time"
)

// How long a read waits for a disconnected drive to come back. USB
// drives that reset themselves reappear within a few seconds, often under
// another name (\\.\PhysicalDrive3 where it was 2, /dev/sdc for sdb).
var (
	reconnectWait = 15 * time.Second
	reconnectPoll = 500 * time.Millisecond
)

// reconnecting is a drive that reopens itself when it disconnects and
// comes back: the same drive by serial number, or by model and size when
// it reports no serial.
type reconnecting struct {
	mu       sync.RWMutex
	dev      Device
	info     Info
	gen      int       // counts reopenings
	failedAt time.Time // when the last wait for it ran out
	list     func() ([]Info, error)
	open     func(Info) (Device, error)
}

// Reconnecting wraps dev so a read that finds the drive gone waits for it
// to return and carries on. When it doesn't return, reads fail with an
// error matching ErrGone.
func Reconnecting(dev Device) Device {
	return &reconnecting{dev: dev, info: dev.Info(), list: List, open: openPhysical}
}

func (r *reconnecting) ReadAt(p []byte, off int64) (int, error) {
	r.mu.RLock()
	dev, gen := r.dev, r.gen
	r.mu.RUnlock()
	n, err := dev.ReadAt(p, off)
	if !IsGone(err) {
		return n, err
	}
	if !r.reopen(gen) {
		return n, gone(err)
	}
	r.mu.RLock()
	dev = r.dev
	r.mu.RUnlock()
	return dev.ReadAt(p, off)
}

// reopen finds the drive again; gen is the generation the failed read
// used, so reads failing together reopen it once.
func (r *reconnecting) reopen(gen int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gen != gen {
		return true
	}
	// After a wait that ran out, look once rather than waiting again on
	// every read.
	deadline := time.Now().Add(reconnectWait)
	if time.Since(r.failedAt) < reconnectWait {
		deadline = time.Now()
	}
	for {
		if info, ok := r.find(); ok {
			if dev, err := r.open(info); err == nil {
				r.dev.Close()
				r.dev = dev
				r.gen++
				r.failedAt = time.Time{}
				return true
			}
		}
		if time.Now().After(deadline) {
			r.failedAt = time.Now()
			return false
		}
		time.Sleep(reconnectPoll)
	}
}

func (r *reconnecting) find() (Info, bool) {
	infos, _ := r.list()
	for _, i := range infos {
		if i.Size != r.info.Size {
			continue
		}
		if r.info.Serial != "" && i.Serial == r.info.Serial ||
			r.info.Serial == "" && i.Serial == "" && i.Model == r.info.Model && i.Path == r.info.Path {
			return i, true
		}
	}
	return Info{}, false
}

func (r *reconnecting) Info() Info  { return r.info }
func (r *reconnecting) Size() int64 { return r.info.Size }
func (r *reconnecting) SectorSize() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dev.SectorSize()
}

func (r *reconnecting) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dev.Close()
}
