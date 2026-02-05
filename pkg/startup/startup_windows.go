//go:build windows

package startup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	registryPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	appName      = "HomeSentry"
)

// hideConsole hides the console window for the command
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

// IsEnabled checks if auto-start is enabled in Windows registry
func IsEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	return err == nil
}

// Enable adds Home Sentry to Windows startup
func Enable() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Use absolute path
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	// Quote the path in case it contains spaces
	value := fmt.Sprintf(`"%s"`, exePath)
	if err := key.SetStringValue(appName, value); err != nil {
		return fmt.Errorf("failed to set registry value: %w", err)
	}

	return nil
}

// Disable removes Home Sentry from Windows startup
func Disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(appName); err != nil {
		// Ignore if value doesn't exist
		if !strings.Contains(err.Error(), "The system cannot find the file specified") {
			return fmt.Errorf("failed to delete registry value: %w", err)
		}
	}

	return nil
}

// PlayWarningSound plays a Windows system warning beep
func PlayWarningSound() {
	// Use PowerShell to play system sound
	cmd := exec.Command("powershell", "-WindowStyle", "Hidden", "-Command",
		"[System.Media.SystemSounds]::Exclamation.Play()")
	hideConsole(cmd)
	cmd.Run()
}

// PlayCriticalSound plays multiple beeps for critical alert
func PlayCriticalSound() {
	// Play beep sound using built-in Windows beep
	cmd := exec.Command("powershell", "-WindowStyle", "Hidden", "-Command",
		"[console]::beep(1000, 500); Start-Sleep -Milliseconds 200; [console]::beep(1000, 500)")
	hideConsole(cmd)
	cmd.Run()
}
