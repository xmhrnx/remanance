//go:build darwin && !ios

package native

import (
	"os/exec"
	"strings"
)

func pickFolder(title string) (string, error) {
	script := `POSIX path of (choose folder with prompt ` + appleString(title) + `)`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", ErrCanceled // osascript fails when the user cancels
	}
	return strings.TrimRight(strings.TrimSpace(string(out)), "/"), nil
}

// appleString quotes s for AppleScript.
func appleString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\`, `"`, `\"`).Replace(s) + `"`
}
