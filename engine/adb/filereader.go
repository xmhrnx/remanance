package adb

// fileReader reads a file on the phone at any offset.
type fileReader struct {
	sh   *shell
	path string
}
