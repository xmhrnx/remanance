package disk

import (
	"errors"
	"fmt"
)

// ErrGone is a drive that stopped answering altogether: unplugged, or
// dropped off the bus by a failing controller. Unlike a bad sector,
// every read after it fails too.
var ErrGone = errors.New("the drive was disconnected")

// IsGone reports whether err means the drive itself is gone, rather than
// one part of it unreadable.
func IsGone(err error) bool {
	return err != nil && (errors.Is(err, ErrGone) || goneErrno(err))
}

// gone wraps a read error that means the drive is gone, keeping the
// system's own words for the report.
func gone(err error) error {
	if errors.Is(err, ErrGone) {
		return err
	}
	return fmt.Errorf("%w (%v)", ErrGone, err)
}
