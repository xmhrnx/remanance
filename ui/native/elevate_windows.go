package native

import (
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func canElevate() bool {
	return !windows.GetCurrentProcessToken().IsElevated()
}

// elevate asks Windows to start the app again as administrator, which
// shows the UAC prompt.
func elevate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	args, _ := syscall.UTF16PtrFromString(strings.Join(quoteArgs(os.Args[1:]), " "))
	dir, _ := os.Getwd()
	cwd, _ := syscall.UTF16PtrFromString(dir)
	return windows.ShellExecute(0, verb, file, args, cwd, windows.SW_NORMAL)
}

func quoteArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = syscall.EscapeArg(a)
	}
	return out
}
