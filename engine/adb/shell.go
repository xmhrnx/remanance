package adb

import (
	"fmt"
	"strings"
	"time"
)

// HasSU tells whether a phone looks rooted: adb runs as root, or there
// is an su to ask, which this doesn't, since a root manager may prompt
// the phone's owner. Asked once per phone.
func HasSU(d Device) bool {
	if d.State != "device" {
		return false
	}
	rootedMu.Lock()
	r, ok := rooted[d.Serial]
	rootedMu.Unlock()
	if ok {
		return r
	}
	out, err := run(d.Serial, "id -u; command -v su", 10*time.Second)
	r = err == nil && (strings.HasPrefix(string(out), "0\n") || strings.Contains(string(out), "/su"))
	rootedMu.Lock()
	rooted[d.Serial] = r
	rootedMu.Unlock()
	return r
}

// root finds how to run commands as root.
func (s *shell) root() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.su != "" {
		return true
	}
	if out, err := run(s.serial, "id -u", 10*time.Second); err == nil && strings.TrimSpace(string(out)) == "0" {
		s.su = "none"
		return true
	}
	// su may wait for the owner to answer a prompt on the phone.
	if out, err := run(s.serial, "su -c 'id -u'", time.Minute); err == nil && strings.TrimSpace(string(out)) == "0" {
		s.su = "su"
		return true
	}
	return false
}

// asRoot wraps a command to run as root.
func (s *shell) asRoot(cmd string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.su == "su" {
		return "su -c " + quote(cmd)
	}
	return cmd
}

// Run runs a command as root.
func (s *shell) Run(command string) (string, error) {
	out, err := run(s.serial, s.asRoot(command), 5*time.Minute)
	return string(out), err
}

// Read fills p from the file at path, with dd, which pads the end of
// the file to a whole block.
func (s *shell) Read(path string, p []byte, off int64) error {
	cmd := fmt.Sprintf("dd if=%s bs=512 skip=%d count=%d conv=noerror,sync 2>/dev/null", quote(path), off/512, len(p)/512)
	out, err := run(s.serial, s.asRoot(cmd), readTimeout)
	if err != nil {
		return err
	}
	if len(out) < len(p) {
		// Past the end of a file that shrank: zeros.
		clear(p[copy(p, out):])
		return nil
	}
	copy(p, out)
	return nil
}
