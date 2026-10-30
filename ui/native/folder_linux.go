//go:build linux && !android

package native

import (
	"errors"
	"os/exec"
	"strings"
)

// pickFolder uses the desktop's own dialog: zenity on GNOME and most
// others, kdialog on KDE.
func pickFolder(title string) (string, error) {
	var cmd *exec.Cmd
	switch {
	case has("zenity"):
		cmd = exec.Command("zenity", "--file-selection", "--directory", "--title="+title)
	case has("kdialog"):
		cmd = exec.Command("kdialog", "--getexistingdirectory", ".", "--title", title)
	case has("qarma"):
		cmd = exec.Command("qarma", "--file-selection", "--directory", "--title="+title)
	default:
		return "", errors.New("no folder dialog found; install zenity or kdialog")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", ErrCanceled
	}
	return strings.TrimSpace(string(out)), nil
}

func has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
