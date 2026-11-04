//go:build android || ios || (!windows && !unix)

package trash

const supported = false

func move(string) error { return ErrNoTrash }

func usage() (Usage, error) { return Usage{}, nil }

func empty() error { return ErrNoTrash }
