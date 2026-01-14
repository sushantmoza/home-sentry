//go:build !windows

package startup

import "fmt"

// IsEnabled checks if auto-start is enabled
func IsEnabled() bool {
	return false
}

// Enable adds Home Sentry to startup
func Enable() error {
	return fmt.Errorf("auto-start not supported on this platform")
}

// Disable removes Home Sentry from startup
func Disable() error {
	return nil
}

// PlayWarningSound plays a warning beep
func PlayWarningSound() {
	// No-op
}

// PlayCriticalSound plays a critical alert sound
func PlayCriticalSound() {
	// No-op
}
