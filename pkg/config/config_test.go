package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"valid IPv4", "192.168.1.1", true},
		{"valid IPv4 zeros", "0.0.0.0", true},
		{"valid IPv4 broadcast", "255.255.255.255", true},
		{"empty string", "", true}, // Empty is allowed
		{"invalid format", "192.168.1", false},
		{"invalid format dots", "192.168.1.1.1", false},
		{"invalid chars", "192.168.1.abc", false},
		{"hostname", "localhost", false},
		{"IPv6", "::1", false}, // We only support IPv4
		{"negative", "-1.0.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateIP(tt.ip)
			if result != tt.expected {
				t.Errorf("ValidateIP(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestDefaultSettings(t *testing.T) {
	defaults := DefaultSettings()

	if defaults.GraceChecks != 5 {
		t.Errorf("Default GraceChecks = %d, want 5", defaults.GraceChecks)
	}
	if defaults.PollInterval != 10 {
		t.Errorf("Default PollInterval = %d, want 10", defaults.PollInterval)
	}
	if defaults.PingTimeoutMs != 500 {
		t.Errorf("Default PingTimeoutMs = %d, want 500", defaults.PingTimeoutMs)
	}
	if defaults.IsPaused != false {
		t.Errorf("Default IsPaused = %v, want false", defaults.IsPaused)
	}
}

func TestLoadSave(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "home-sentry-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override settings path for test
	origAppData := os.Getenv("APPDATA")
	os.Setenv("APPDATA", tmpDir)
	defer os.Setenv("APPDATA", origAppData)

	// Create the HomeSentry directory
	os.MkdirAll(filepath.Join(tmpDir, "HomeSentry"), 0755)

	// Test saving
	settings := Settings{
		HomeSSID:      "TestWiFi",
		PhoneIP:       "192.168.1.100",
		IsPaused:      true,
		GraceChecks:   3,
		PollInterval:  5,
		PingTimeoutMs: 1000,
	}

	err = Save(settings)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Test loading
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.HomeSSID != settings.HomeSSID {
		t.Errorf("Loaded HomeSSID = %q, want %q", loaded.HomeSSID, settings.HomeSSID)
	}
	if loaded.PhoneIP != settings.PhoneIP {
		t.Errorf("Loaded PhoneIP = %q, want %q", loaded.PhoneIP, settings.PhoneIP)
	}
	if loaded.IsPaused != settings.IsPaused {
		t.Errorf("Loaded IsPaused = %v, want %v", loaded.IsPaused, settings.IsPaused)
	}
	if loaded.GraceChecks != settings.GraceChecks {
		t.Errorf("Loaded GraceChecks = %d, want %d", loaded.GraceChecks, settings.GraceChecks)
	}
}

func TestUpdate(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "home-sentry-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origAppData := os.Getenv("APPDATA")
	os.Setenv("APPDATA", tmpDir)
	defer os.Setenv("APPDATA", origAppData)

	os.MkdirAll(filepath.Join(tmpDir, "HomeSentry"), 0755)

	// Test update with valid IP
	err = Update("MyWiFi", "192.168.1.50")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	loaded, _ := Load()
	if loaded.HomeSSID != "MyWiFi" {
		t.Errorf("Updated HomeSSID = %q, want %q", loaded.HomeSSID, "MyWiFi")
	}
	if loaded.PhoneIP != "192.168.1.50" {
		t.Errorf("Updated PhoneIP = %q, want %q", loaded.PhoneIP, "192.168.1.50")
	}

	// Test update with invalid IP
	err = Update("", "invalid-ip")
	if err == nil {
		t.Error("Update() with invalid IP should return error")
	}
}

func TestLoadMinimumValues(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "home-sentry-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origAppData := os.Getenv("APPDATA")
	os.Setenv("APPDATA", tmpDir)
	defer os.Setenv("APPDATA", origAppData)

	hsDir := filepath.Join(tmpDir, "HomeSentry")
	os.MkdirAll(hsDir, 0755)

	// Write settings with invalid minimum values
	settingsPath := filepath.Join(hsDir, "settings.json")
	content := `{"grace_checks": 0, "poll_interval_sec": 0, "ping_timeout_ms": 50}`
	os.WriteFile(settingsPath, []byte(content), 0644)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Values should be corrected to minimums
	if loaded.GraceChecks < 1 {
		t.Errorf("GraceChecks should be >= 1, got %d", loaded.GraceChecks)
	}
	if loaded.PollInterval < 1 {
		t.Errorf("PollInterval should be >= 1, got %d", loaded.PollInterval)
	}
	if loaded.PingTimeoutMs < 100 {
		t.Errorf("PingTimeoutMs should be >= 100, got %d", loaded.PingTimeoutMs)
	}
}
