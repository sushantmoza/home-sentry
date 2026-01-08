package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

type Settings struct {
	HomeSSID      string `json:"home_ssid"`
	PhoneIP       string `json:"phone_ip"`
	IsPaused      bool   `json:"is_paused"`
	GraceChecks   int    `json:"grace_checks"`
	PollInterval  int    `json:"poll_interval_sec"`
	PingTimeoutMs int    `json:"ping_timeout_ms"`
}

// DefaultSettings returns settings with sensible defaults
func DefaultSettings() Settings {
	return Settings{
		HomeSSID:      "",
		PhoneIP:       "",
		IsPaused:      false,
		GraceChecks:   5,
		PollInterval:  10,
		PingTimeoutMs: 500,
	}
}

// getSettingsPath returns the path to the settings file in %APPDATA%\HomeSentry
func getSettingsPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		// Fallback to current directory on non-Windows or if APPDATA not set
		return "settings.json", nil
	}

	dir := filepath.Join(appData, "HomeSentry")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(dir, "settings.json"), nil
}

// ValidateIP checks if the given string is a valid IPv4 address
func ValidateIP(ip string) bool {
	if ip == "" {
		return true // Empty is allowed (means no device configured)
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() != nil
}

func Load() (Settings, error) {
	path, err := getSettingsPath()
	if err != nil {
		return DefaultSettings(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults if file doesn't exist
			return DefaultSettings(), nil
		}
		return DefaultSettings(), err
	}

	settings := DefaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultSettings(), err
	}

	// Ensure minimum values
	if settings.GraceChecks < 1 {
		settings.GraceChecks = 5
	}
	if settings.PollInterval < 1 {
		settings.PollInterval = 10
	}
	if settings.PingTimeoutMs < 100 {
		settings.PingTimeoutMs = 500
	}

	return settings, nil
}

func Save(settings Settings) error {
	path, err := getSettingsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Update(ssid, ip string) error {
	settings, _ := Load()
	if ssid != "" {
		settings.HomeSSID = ssid
	}
	if ip != "" {
		if !ValidateIP(ip) {
			return fmt.Errorf("invalid IP address: %s", ip)
		}
		settings.PhoneIP = ip
	}

	fmt.Printf("Updating settings: %+v\n", settings)
	return Save(settings)
}

func SetPaused(paused bool) error {
	settings, _ := Load()
	settings.IsPaused = paused
	fmt.Printf("Updating paused status: %v\n", paused)
	return Save(settings)
}

// GetSettingsPath exposes the settings path for display purposes
func GetSettingsPath() string {
	path, _ := getSettingsPath()
	return path
}
