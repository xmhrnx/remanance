//go:build (linux && !android) || (darwin && !ios)

package native

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func canElevate() bool {
	if os.Geteuid() == 0 {
		return false
	}
	if runtime.GOOS == "darwin" {
		return true
	}
	_, err := exec.LookPath("pkexec")
	return err == nil
}

// elevate starts the app again as root: through pkexec on Linux, which
// asks for the password in the desktop's own dialog, passing on what a
// window needs to reach the display; through an administrator prompt on
// macOS.
func elevate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		script := `do shell script quoted form of ` + appleQuote(exe) + ` & " >/dev/null 2>&1 &" with administrator privileges`
		return exec.Command("osascript", "-e", script).Start()
	}
	args := []string{"env"}
	for _, k := range []string{"DISPLAY", "XAUTHORITY", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "XDG_SESSION_TYPE", "DBUS_SESSION_BUS_ADDRESS", "LANG"} {
		if v, ok := os.LookupEnv(k); ok {
			args = append(args, k+"="+v)
		}
	}
	args = append(args, exe)
	return exec.Command("pkexec", args...).Start()
}

func appleQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\`, `"`, `\"`).Replace(s) + `"`
}
