//go:build (!windows && !linux && !darwin) || android || ios

package native

func canElevate() bool { return false }

func elevate() error { return ErrUnsupported }
