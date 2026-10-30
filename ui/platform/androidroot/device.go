package androidroot

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
)

// suPaths are where root solutions put su.
var suPaths = []string{"/system/bin/su", "/system/xbin/su", "/sbin/su", "/su/bin/su", "/debug_ramdisk/su", "/system_ext/bin/su"}

// hasSU reports whether the phone looks rooted, without asking su
// anything, which would prompt the user.
func hasSU() bool {
	for _, p := range suPaths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	_, err := exec.LookPath("su")
	return err == nil
}

// provider lists and opens devices through one root shell, started the
// first time the drives are listed.
type provider struct {
	mu    sync.Mutex
	shell *session
	start func() (*session, error)
}

func (p *provider) session() (*session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.shell != nil && p.shell.dead == nil {
		return p.shell, nil
	}
	s, err := p.start()
	if err != nil {
		return nil, err
	}
	p.shell = s
	return s, nil
}

func (p *provider) List() ([]disk.Info, error) {
	s, err := p.session()
	if err != nil {
		return nil, nil // not rooted, or root refused: nothing to add
	}
	out, err := s.run(listCommand, 30*time.Second)
	if err != nil {
		return nil, nil
	}
	return parse(out), nil
}

func (p *provider) Handles(id string) bool { return strings.HasPrefix(id, prefix) }

func (p *provider) Open(info disk.Info) (disk.Device, error) {
	s, err := p.session()
	if err != nil {
		return nil, err
	}
	bs := 4096
	if info.Size%4096 != 0 {
		bs = 512
	}
	d := &device{info: info}
	d.r = disk.Aligned(blocks{s, info.Path, bs}, bs, info.Size)
	// A first read says whether the device opens at all.
	probe := make([]byte, 512)
	if _, err := d.ReadAt(probe, 0); err != nil {
		return nil, err
	}
	return d, nil
}

// blocks reads whole blocks through the shell.
type blocks struct {
	s   *session
	dev string
	bs  int
}

func (b blocks) ReadAt(p []byte, off int64) (int, error) {
	// Long reads go in pieces, so one dd never holds the shell long.
	const most = 4 << 20
	for n := 0; n < len(p); n += most {
		k := min(most, len(p)-n)
		if err := b.s.read(b.dev, b.bs, p[n:n+k], off+int64(n)); err != nil {
			return n, err
		}
	}
	return len(p), nil
}

type device struct {
	info disk.Info
	r    io.ReaderAt
}

func (d *device) ReadAt(p []byte, off int64) (int, error) { return d.r.ReadAt(p, off) }
func (d *device) Info() disk.Info                         { return d.info }
func (d *device) Size() int64                             { return d.info.Size }
func (d *device) SectorSize() int                         { return 512 }

// Close leaves the shell open for the next scan, so su doesn't ask
// again.
func (d *device) Close() error { return nil }
