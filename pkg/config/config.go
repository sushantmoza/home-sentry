package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DetectionType specifies how to detect the phone
type DetectionType string

const (
	DetectionTypeIP  DetectionType = "ip"
	DetectionTypeMAC DetectionType = "mac"
)

type Settings struct {
	HomeSSID       string        `json:"home_ssid"`
	PhoneIP        string        `json:"phone_ip"`
	PhoneMAC       string        `json:"phone_mac"`
	DetectionType  DetectionType `json:"detection_type"`
	IsPaused       bool          `json:"is_paused"`
	GraceChecks    int           `json:"grace_checks"`
	PollInterval   int           `json:"poll_interval_sec"`
	PingTimeoutMs  int           `json:"ping_timeout_ms"`
	ShutdownDelay  int           `json:"shutdown_delay_sec"`
	ShutdownPIN    string        `json:"shutdown_pin"`
	RequirePIN     bool          `json:"require_pin"`
	ShutdownAction string        `json:"shutdown_action"`
}

// DefaultSettings returns settings with sensible defaults
func DefaultSettings() Settings {
	return Settings{
		HomeSSID:       "",
		PhoneIP:        "",
		PhoneMAC:       "",
		DetectionType:  DefaultDetectionType,
		IsPaused:       false,
		GraceChecks:    DefaultGraceChecks,
		PollInterval:   DefaultPollInterval,
		PingTimeoutMs:  DefaultPingTimeoutMs,
		ShutdownDelay:  DefaultShutdownDelay,
		ShutdownPIN:    "",
		RequirePIN:     false,
		ShutdownAction: DefaultShutdownAction,
	}
}

// getSettingsPath returns the path to the settings file in %APPDATA%\HomeSentry
func getSettingsPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "settings.json", nil
	}

	dir := filepath.Join(appData, "HomeSentry")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(dir, "settings.json"), nil
}

// ValidateIP checks if the given string is a valid IPv4 address
func ValidateIP(ip string) bool {
	if ip == "" {
		return true
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() != nil
}

// ValidateMAC checks if the given string is a valid MAC address
func ValidateMAC(mac string) bool {
	if mac == "" {
		return true
	}
	// Support both formats: 00:11:22:33:44:55 and 00-11-22-33-44-55
	macRegex := regexp.MustCompile(`^([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}$`)
	return macRegex.MatchString(mac)
}

// NormalizeMAC converts MAC address to lowercase with dashes (Windows ARP format)
func NormalizeMAC(mac string) string {
	if mac == "" {
		return ""
	}
	// Replace colons with dashes and convert to lowercase
	result := strings.ToLower(mac)
	result = strings.ReplaceAll(result, ":", "-")
	return result
}

// ValidatePIN checks if the given string is a valid PIN (4-8 digits)
func ValidatePIN(pin string) bool {
	if pin == "" {
		return true // Empty PIN is valid (disables PIN requirement)
	}
	if len(pin) < MinPINLength || len(pin) > MaxPINLength {
		return false
	}
	// PIN must be all digits
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ValidateShutdownAction checks if the action is valid
func ValidateShutdownAction(action string) bool {
	switch action {
	case ShutdownActionShutdown, ShutdownActionHibernate, ShutdownActionLock, ShutdownActionSleep:
		return true
	default:
		return false
	}
}

// VerifyPIN checks if the provided PIN matches the stored PIN
func (s Settings) VerifyPIN(pin string) bool {
	if !s.RequirePIN || s.ShutdownPIN == "" {
		return true // No PIN required
	}
	return s.ShutdownPIN == pin
}

func Load() (Settings, error) {
	path, err := getSettingsPath()
	if err != nil {
		return DefaultSettings(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}
		return DefaultSettings(), err
	}

	settings := DefaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultSettings(), err
	}

	// Decrypt sensitive fields
	decrypted, err := DecryptSettings(&settings)
	if err != nil {
		// If decryption fails, might be unencrypted legacy settings
		// Log the error but continue with potentially unencrypted data
		fmt.Printf("Warning: Could not decrypt settings (may be legacy format): %v\n", err)
		decrypted = &settings
	}

	// Ensure minimum values
	if decrypted.GraceChecks < 1 {
		decrypted.GraceChecks = 5
	}
	if decrypted.PollInterval < 1 {
		decrypted.PollInterval = 10
	}
	if decrypted.PingTimeoutMs < 100 {
		decrypted.PingTimeoutMs = 500
	}
	if decrypted.ShutdownDelay < 5 {
		decrypted.ShutdownDelay = 10
	}
	// Default to IP detection if not set
	if decrypted.DetectionType == "" {
		decrypted.DetectionType = DetectionTypeIP
	}
	if decrypted.ShutdownAction == "" {
		decrypted.ShutdownAction = DefaultShutdownAction
	}

	return *decrypted, nil
}

func Save(settings Settings) error {
	path, err := getSettingsPath()
	if err != nil {
		return err
	}

	// Encrypt sensitive fields before saving
	encrypted, err := EncryptSettings(&settings)
	if err != nil {
		return fmt.Errorf("failed to encrypt settings: %w", err)
	}

	data, err := json.MarshalIndent(encrypted, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func Update(ssid, mac string) error {
	settings, _ := Load()
	if ssid != "" {
		sanitizedSSID, err := SanitizeSSID(ssid)
		if err != nil {
			return err
		}
		settings.HomeSSID = sanitizedSSID
	}
	if mac != "" {
		sanitizedMAC, err := SanitizeMAC(mac)
		if err != nil {
			return err
		}
		settings.PhoneMAC = sanitizedMAC
		settings.DetectionType = DetectionTypeMAC
	}
	return Save(settings)
}

// UpdateDevice updates both IP and MAC with the specified detection type
func UpdateDevice(ip, mac string, detectionType DetectionType) error {
	settings, _ := Load()

	if ip != "" {
		sanitizedIP, err := SanitizeIP(ip)
		if err != nil {
			return err
		}
		settings.PhoneIP = sanitizedIP
	}

	if mac != "" {
		sanitizedMAC, err := SanitizeMAC(mac)
		if err != nil {
			return err
		}
		settings.PhoneMAC = sanitizedMAC
	}

	if detectionType != "" {
		settings.DetectionType = detectionType
	}

	fmt.Printf("Updating device settings: IP=%s, MAC=%s, Type=%s\n", settings.PhoneIP, settings.PhoneMAC, settings.DetectionType)
	return Save(settings)
}

// SetDetectionType sets the detection type (ip or mac)
func SetDetectionType(detectionType DetectionType) error {
	settings, _ := Load()
	settings.DetectionType = detectionType
	fmt.Printf("Updating detection type: %s\n", detectionType)
	return Save(settings)
}

func SetPaused(paused bool) error {
	settings, _ := Load()
	settings.IsPaused = paused
	fmt.Printf("Updating paused status: %v\n", paused)
	return Save(settings)
}

func SetShutdownDelay(seconds int) error {
	settings, _ := Load()
	if seconds < ShutdownMinDelay {
		return fmt.Errorf("shutdown delay must be at least %d seconds", ShutdownMinDelay)
	}
	if seconds > ShutdownMaxDelay {
		return fmt.Errorf("shutdown delay must be at most %d seconds", ShutdownMaxDelay)
	}
	settings.ShutdownDelay = seconds
	fmt.Printf("Updating shutdown delay: %d seconds\n", seconds)
	return Save(settings)
}

// SetShutdownPIN sets the PIN required for shutdown confirmation
func SetShutdownPIN(pin string) error {
	if !ValidatePIN(pin) {
		return fmt.Errorf("PIN must be %d-%d digits", MinPINLength, MaxPINLength)
	}
	settings, _ := Load()
	settings.ShutdownPIN = pin
	settings.RequirePIN = pin != ""
	fmt.Printf("Updating shutdown PIN: %s\n", func() string {
		if pin == "" {
			return "disabled"
		}
		return "enabled"
	}())
	return Save(settings)
}

// SetRequirePIN toggles whether a PIN is required for shutdown
func SetRequirePIN(require bool) error {
	settings, _ := Load()
	settings.RequirePIN = require
	fmt.Printf("Updating PIN requirement: %v\n", require)
	return Save(settings)
}

// SetShutdownAction sets the action to take when protection triggers
func SetShutdownAction(action string) error {
	if !ValidateShutdownAction(action) {
		return fmt.Errorf("invalid shutdown action: %s (valid: shutdown, hibernate, lock, sleep)", action)
	}
	settings, _ := Load()
	settings.ShutdownAction = action
	fmt.Printf("Updating shutdown action: %s\n", action)
	return Save(settings)
}

// GetSettingsPath exposes the settings path for display purposes
func GetSettingsPath() string {
	path, _ := getSettingsPath()
	return path
}

// HasDeviceConfigured returns true if a device is configured for monitoring
func (s Settings) HasDeviceConfigured() bool {
	switch s.DetectionType {
	case DetectionTypeMAC:
		return s.PhoneMAC != ""
	default:
		return s.PhoneIP != "" && s.PhoneIP != "0.0.0.0"
	}
}

// GetDeviceIdentifier returns the configured device identifier based on detection type
func (s Settings) GetDeviceIdentifier() string {
	switch s.DetectionType {
	case DetectionTypeMAC:
		return s.PhoneMAC
	default:
		return s.PhoneIP
	}
}
