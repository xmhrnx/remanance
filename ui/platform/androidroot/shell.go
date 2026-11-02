// Package androidroot reads a rooted Android phone's own storage: the
// userdata partition and a memory card in the phone's slot, which no app
// may open without root. A shell from su stays open and serves reads
// with dd; su asks the user once.
//
// Android 10 and later encrypt userdata file by file and throw away a
// deleted file's key, so its sectors only give back what an older or
// unencrypted phone left. A memory card set up as portable storage is
// not encrypted, and reads like any card.
package androidroot

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// session is a root shell kept open for reading block devices.
type session struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  *bufio.Reader
	dead error
}

// readTimeout is how long one read may take before the shell is
// presumed stuck and killed.
const readTimeout = 60 * time.Second

// errShell means the root shell died or stopped answering.
var errShell = errors.New("the root shell stopped answering")

// start runs su and checks the shell it gives is root.
func start(name string) (*session, error) {
	s, err := spawn(name)
	if err != nil {
		return nil, err
	}
	// The first command waits for su's prompt, so it gets longer.
	if who, err := s.run("id -u", 5*time.Minute); err != nil || strings.TrimSpace(who) != "0" {
		s.close()
		if err == nil {
			err = fmt.Errorf("su gave a shell without root (uid %s)", strings.TrimSpace(who))
		}
		return nil, err
	}
	return s, nil
}

// run runs a shell command and returns what it printed.
func (s *session) run(command string, timeout time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dead != nil {
		return "", s.dead
	}
	const end = "--datadoc-end--"
	stop := s.watch(timeout)
	defer stop()
	if _, err := fmt.Fprintf(s.in, "%s\necho %s\n", command, end); err != nil {
		return "", s.fail(err)
	}
	var b strings.Builder
	for {
		line, err := s.out.ReadString('\n')
		if err != nil {
			return "", s.fail(err)
		}
		if strings.TrimSpace(line) == end {
			return b.String(), nil
		}
		b.WriteString(line)
	}
}

// read fills p with the bytes of dev at off, both multiples of bs. dd
// pads blocks it can't read with zeros, so it always writes len(p)
// bytes, as long as the range is inside the device.
func (s *session) read(dev string, bs int, p []byte, off int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dead != nil {
		return s.dead
	}
	stop := s.watch(readTimeout)
	defer stop()
	cmd := fmt.Sprintf("dd if=%s bs=%d skip=%d count=%d conv=noerror,sync 2>/dev/null\n", quote(dev), bs, off/int64(bs), len(p)/bs)
	if _, err := io.WriteString(s.in, cmd); err != nil {
		return s.fail(err)
	}
	if _, err := io.ReadFull(s.out, p); err != nil {
		return s.fail(err)
	}
	return nil
}

// watch kills the shell if the command running doesn't finish within d,
// so a stuck read fails instead of hanging the scan.
func (s *session) watch(d time.Duration) (stop func()) {
	t := time.AfterFunc(d, func() { s.cmd.Process.Kill() })
	return func() { t.Stop() }
}

func (s *session) fail(err error) error {
	s.dead = fmt.Errorf("%w (%v)", errShell, err)
	s.cmd.Process.Kill()
	return s.dead
}

func (s *session) close() {
	s.in.Close()
	s.cmd.Process.Kill()
	s.cmd.Wait()
}

// quote quotes a path for the shell.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// spawn starts a shell.
func spawn(name string) (*session, error) {
	cmd := exec.Command(name)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &session{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<20)}, nil
}
