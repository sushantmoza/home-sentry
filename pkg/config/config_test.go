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

func TestValidateMAC(t *testing.T) {
	tests := []struct {
		name     string
		mac      string
		expected bool
	}{
		{"valid colon format", "AA:BB:CC:DD:EE:FF", true},
		{"valid dash format", "AA-BB-CC-DD-EE-FF", true},
		{"valid lowercase colon", "aa:bb:cc:dd:ee:ff", true},
		{"valid lowercase dash", "aa-bb-cc-dd-ee-ff", true},
		{"valid mixed case", "Aa:Bb:Cc:Dd:Ee:Ff", true},
		{"empty string", "", true}, // Empty is allowed
		{"too short", "AA:BB:CC:DD:EE", false},
		{"too long", "AA:BB:CC:DD:EE:FF:GG", false},
		{"invalid chars", "GG:HH:II:JJ:KK:LL", false},
		{"no separator", "AABBCCDDEEFF", false},
		{"mixed separators", "AA:BB-CC:DD-EE:FF", true}, // Mixed is allowed, we normalize anyway
		{"IP address", "192.168.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateMAC(tt.mac)
			if result != tt.expected {
				t.Errorf("ValidateMAC(%q) = %v, want %v", tt.mac, result, tt.expected)
			}
		})
	}
}

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		name     string
		mac      string
		expected string
	}{
		{"colon to dash", "AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff"},
		{"already dash", "AA-BB-CC-DD-EE-FF", "aa-bb-cc-dd-ee-ff"},
		{"lowercase colon", "aa:bb:cc:dd:ee:ff", "aa-bb-cc-dd-ee-ff"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeMAC(tt.mac)
			if result != tt.expected {
				t.Errorf("NormalizeMAC(%q) = %q, want %q", tt.mac, result, tt.expected)
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
	if err := os.MkdirAll(filepath.Join(tmpDir, "HomeSentry"), 0755); err != nil {
		t.Fatal(err)
	}

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

	if err := os.MkdirAll(filepath.Join(tmpDir, "HomeSentry"), 0755); err != nil {
		t.Fatal(err)
	}

	// Test update with valid MAC
	err = Update("MyWiFi", "AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	loaded, _ := Load()
	if loaded.HomeSSID != "MyWiFi" {
		t.Errorf("Updated HomeSSID = %q, want %q", loaded.HomeSSID, "MyWiFi")
	}
	// MAC should be normalized to lowercase with dashes
	if loaded.PhoneMAC != "aa-bb-cc-dd-ee-ff" {
		t.Errorf("Updated PhoneMAC = %q, want %q", loaded.PhoneMAC, "aa-bb-cc-dd-ee-ff")
	}
	if loaded.DetectionType != DetectionTypeMAC {
		t.Errorf("DetectionType = %q, want %q", loaded.DetectionType, DetectionTypeMAC)
	}

	// Test update with invalid MAC
	err = Update("", "invalid-mac")
	if err == nil {
		t.Error("Update() with invalid MAC should return error")
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
	if err := os.MkdirAll(hsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write settings with invalid minimum values
	settingsPath := filepath.Join(hsDir, "settings.json")
	content := `{"grace_checks": 0, "poll_interval_sec": 0, "ping_timeout_ms": 50}`
	if err := os.WriteFile(settingsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

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
