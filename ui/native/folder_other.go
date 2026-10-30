//go:build (!windows && !linux && !darwin) || android || ios

package native

// Phones save into their own Downloads or Files folders instead.
func pickFolder(string) (string, error) { return "", ErrUnsupported }
