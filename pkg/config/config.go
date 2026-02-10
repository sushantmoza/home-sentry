package config

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// settingsMu protects concurrent access to the settings file.
// All Load/Save operations must hold this lock to prevent read-modify-write races.
var settingsMu sync.Mutex

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

// ValidateSettings validates and sanitizes all settings fields loaded from disk.
// Invalid fields are reset to safe defaults rather than rejecting the entire file.
func ValidateSettings(s *Settings) []string {
	var warnings []string

	// Validate and sanitize SSID
	if s.HomeSSID != "" {
		sanitized, err := SanitizeSSID(s.HomeSSID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("HomeSSID invalid, reset to empty: %v", err))
			s.HomeSSID = ""
		} else {
			s.HomeSSID = sanitized
		}
	}

	// Validate and sanitize PhoneIP
	if s.PhoneIP != "" {
		sanitized, err := SanitizeIP(s.PhoneIP)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("PhoneIP invalid, reset to empty: %v", err))
			s.PhoneIP = ""
		} else {
			s.PhoneIP = sanitized
		}
	}

	// Validate and sanitize PhoneMAC
	if s.PhoneMAC != "" {
		sanitized, err := SanitizeMAC(s.PhoneMAC)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("PhoneMAC invalid, reset to empty: %v", err))
			s.PhoneMAC = ""
		} else {
			s.PhoneMAC = sanitized
		}
	}

	// Validate ShutdownPIN
	if s.ShutdownPIN != "" {
		sanitized, err := SanitizePIN(s.ShutdownPIN)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("ShutdownPIN invalid, reset to empty: %v", err))
			s.ShutdownPIN = ""
			s.RequirePIN = false
		} else {
			s.ShutdownPIN = sanitized
		}
	}

	// Validate DetectionType
	if s.DetectionType != DetectionTypeIP && s.DetectionType != DetectionTypeMAC {
		warnings = append(warnings, fmt.Sprintf("DetectionType invalid (%s), reset to default", s.DetectionType))
		s.DetectionType = DefaultDetectionType
	}

	// Validate ShutdownAction
	if !ValidateShutdownAction(s.ShutdownAction) {
		warnings = append(warnings, fmt.Sprintf("ShutdownAction invalid (%s), reset to default", s.ShutdownAction))
		s.ShutdownAction = DefaultShutdownAction
	}

	// Validate numeric ranges
	if s.GraceChecks < MinGraceChecks || s.GraceChecks > MaxGraceChecks {
		warnings = append(warnings, fmt.Sprintf("GraceChecks out of range (%d), reset to default", s.GraceChecks))
		s.GraceChecks = DefaultGraceChecks
	}
	if s.PollInterval < MinPollInterval || s.PollInterval > MaxPollInterval {
		warnings = append(warnings, fmt.Sprintf("PollInterval out of range (%d), reset to default", s.PollInterval))
		s.PollInterval = DefaultPollInterval
	}
	if s.ShutdownDelay < ShutdownMinDelay || s.ShutdownDelay > ShutdownMaxDelay {
		warnings = append(warnings, fmt.Sprintf("ShutdownDelay out of range (%d), reset to default", s.ShutdownDelay))
		s.ShutdownDelay = DefaultShutdownDelay
	}

	return warnings
}

// VerifyPIN checks if the provided PIN matches the stored PIN using constant-time comparison
func (s Settings) VerifyPIN(pin string) bool {
	if !s.RequirePIN || s.ShutdownPIN == "" {
		return true // No PIN required
	}
	return subtle.ConstantTimeCompare([]byte(s.ShutdownPIN), []byte(pin)) == 1
}

func Load() (Settings, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	return loadLocked()
}

// loadLocked performs the actual load. Caller must hold settingsMu.
func loadLocked() (Settings, error) {
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
		// Continue with potentially unencrypted data but log the warning
		decrypted = &settings
	}

	// Validate and sanitize all fields loaded from disk
	ValidateSettings(decrypted)

	// Ensure minimum values for fields not covered by ValidateSettings range checks
	if decrypted.PingTimeoutMs < 100 {
		decrypted.PingTimeoutMs = DefaultPingTimeoutMs
	}

	return *decrypted, nil
}

func Save(settings Settings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	return saveLocked(settings)
}

// saveLocked performs the actual save with atomic write. Caller must hold settingsMu.
func saveLocked(settings Settings) error {
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

	// Atomic write: write to temp file, then rename to avoid corruption on crash
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions before rename
	if err := os.Chmod(tmpPath, 0600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename (on Windows, os.Rename replaces existing files)
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func Update(ssid, mac string) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	settings, err := loadLocked()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
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
	return saveLocked(settings)
}

// UpdateDevice updates both IP and MAC with the specified detection type
func UpdateDevice(ip, mac string, detectionType DetectionType) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	settings, err := loadLocked()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

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

	return saveLocked(settings)
}

// SetDetectionType sets the detection type (ip or mac)
func SetDetectionType(detectionType DetectionType) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	settings, err := loadLocked()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	settings.DetectionType = detectionType
	return saveLocked(settings)
}

func SetPaused(paused bool) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	settings, err := loadLocked()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	settings.IsPaused = paused
	return saveLocked(settings)
}

func SetShutdownDelay(seconds int) error {
	if seconds < ShutdownMinDelay {
		return fmt.Errorf("shutdown delay must be at least %d seconds", ShutdownMinDelay)
	}
	if seconds > ShutdownMaxDelay {
		return fmt.Errorf("shutdown delay must be at most %d seconds", ShutdownMaxDelay)
	}

	settingsMu.Lock()
	defer settingsMu.Unlock()

	settings, err := loadLocked()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	settings.ShutdownDelay = seconds
	return saveLocked(settings)
}

// SetShutdownPIN sets the PIN required for shutdown confirmation
func SetShutdownPIN(pin string) error {
	if !ValidatePIN(pin) {
		return fmt.Errorf("PIN must be %d-%d digits", MinPINLength, MaxPINLength)
	}

	settingsMu.Lock()
	defer settingsMu.Unlock()

	settings, err := loadLocked()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	settings.ShutdownPIN = pin
	settings.RequirePIN = pin != ""
	return saveLocked(settings)
}

// SetRequirePIN toggles whether a PIN is required for shutdown
func SetRequirePIN(require bool) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	settings, err := loadLocked()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	settings.RequirePIN = require
	return saveLocked(settings)
}

// SetShutdownAction sets the action to take when protection triggers
func SetShutdownAction(action string) error {
	if !ValidateShutdownAction(action) {
		return fmt.Errorf("invalid shutdown action: %s (valid: shutdown, hibernate, lock, sleep)", action)
	}

	settingsMu.Lock()
	defer settingsMu.Unlock()

	settings, err := loadLocked()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	settings.ShutdownAction = action
	return saveLocked(settings)
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
