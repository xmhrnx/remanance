//go:build !unix && !windows

package cleanup

import "errors"

func free(string) (Space, error) { return Space{}, errors.New("free space isn't known here") }

func mine(string) bool { return true }
