package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
)

// DetectionType specifies how to detect the phone
type DetectionType string

const (
	DetectionTypeIP  DetectionType = "ip"
	DetectionTypeMAC DetectionType = "mac"
)

type Settings struct {
	HomeSSID      string        `json:"home_ssid"`
	PhoneIP       string        `json:"phone_ip"`
	PhoneMAC      string        `json:"phone_mac"`
	DetectionType DetectionType `json:"detection_type"`
	IsPaused      bool          `json:"is_paused"`
	GraceChecks   int           `json:"grace_checks"`
	PollInterval  int           `json:"poll_interval_sec"`
	PingTimeoutMs int           `json:"ping_timeout_ms"`
}

// DefaultSettings returns settings with sensible defaults
func DefaultSettings() Settings {
	return Settings{
		HomeSSID:      "",
		PhoneIP:       "",
		PhoneMAC:      "",
		DetectionType: DetectionTypeIP,
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
	// Replace colons with dashes and lowercase
	result := regexp.MustCompile(`[:-]`).ReplaceAllString(mac, "-")
	return result
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
	// Default to IP detection if not set
	if settings.DetectionType == "" {
		settings.DetectionType = DetectionTypeIP
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

// UpdateDevice updates both IP and MAC with the specified detection type
func UpdateDevice(ip, mac string, detectionType DetectionType) error {
	settings, _ := Load()

	if ip != "" {
		if !ValidateIP(ip) {
			return fmt.Errorf("invalid IP address: %s", ip)
		}
		settings.PhoneIP = ip
	}

	if mac != "" {
		if !ValidateMAC(mac) {
			return fmt.Errorf("invalid MAC address: %s", mac)
		}
		settings.PhoneMAC = NormalizeMAC(mac)
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
