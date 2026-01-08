//go:build !windows
// +build !windows

package network

import "os/exec"

// HideConsole is a no-op on non-Windows platforms
func HideConsole(cmd *exec.Cmd) {
	// Nothing to do on non-Windows
}
