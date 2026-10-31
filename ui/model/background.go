package model

import (
	"sync"

	"github.com/mehran9675/giode"
)

// keepRunning keeps a long job going while the app is out of sight: on
// Android a foreground service, with a notification that brings the app
// back, so the system doesn't freeze or end it mid-scan; on desktops it
// holds off idle sleep. The returned func ends it.
func keepRunning(app *giode.App, reason string) func() {
	release, err := app.KeepAlive(reason)
	if err != nil {
		return func() {}
	}
	return release
}

// latest hands a job's progress to the UI with at most one update
// waiting. While the app is in the background no frames run, so posting
// every update would leave a backlog for the first frame to work
// through when the app comes back, and keep the window from showing.
type latest[T any] struct {
	mu      sync.Mutex
	v       T
	waiting bool
}

// post shows v through set, unless an update is already waiting, in
// which case that one shows v instead.
func (l *latest[T]) post(app *giode.App, v T, set func(T)) {
	l.mu.Lock()
	l.v = v
	waiting := l.waiting
	l.waiting = true
	l.mu.Unlock()
	if waiting {
		return
	}
	app.Post(func() {
		l.mu.Lock()
		v := l.v
		l.waiting = false
		l.mu.Unlock()
		set(v)
	})
}
