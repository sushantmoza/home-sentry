//go:build windows
// +build windows

package network

import (
	"os/exec"
	"syscall"
)

// HideConsole configures a command to run without showing a console window on Windows
func HideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
