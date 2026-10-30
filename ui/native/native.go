// Package native does the few things the app needs from the operating
// system that the UI library doesn't: picking a folder, and starting
// again with the rights to read drives directly.
package native

import "errors"

// ErrCanceled is returned when the user closes a picker without choosing.
var ErrCanceled = errors.New("canceled")

// ErrUnsupported is returned where a platform has no such thing.
var ErrUnsupported = errors.New("not supported on this platform")

// PickFolder asks the user for a folder, starting the dialog with title.
// It blocks; call it off the UI goroutine.
func PickFolder(title string) (string, error) { return pickFolder(title) }

// CanElevate reports whether Elevate can restart the app with
// administrator rights, and it doesn't have them yet.
func CanElevate() bool { return canElevate() }

// Elevate starts a new copy of the app with administrator rights. On
// success the caller should quit.
func Elevate() error { return elevate() }
