//go:build !windows

package adb

import "os/exec"

func hide(*exec.Cmd) {}
