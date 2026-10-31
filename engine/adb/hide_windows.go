package adb

import (
	"os/exec"
	"syscall"
)

// hide keeps adb's console window from flashing up.
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
