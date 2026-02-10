package startup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	registryPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	appName      = "HomeSentry"
)

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

// Toggle switches auto-start on/off
func Toggle() (enabled bool, err error) {
	if IsEnabled() {
		err = Disable()
		return false, err
	}
	err = Enable()
	return true, err
}
