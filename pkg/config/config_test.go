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

func TestSanitizeHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantErr  bool
		expected string
	}{
		{"valid hostname", "myhost.local", false, "myhost.local"},
		{"empty string", "", false, ""},
		{"with control chars", "host\x00name", false, "hostname"},
		{"with format string", "host%sname", false, "host%%sname"},
		{"with dangerous chars", "host<script>", false, "hostscript"},
		{"very long hostname", string(make([]byte, 300)), false, "Unknown"},
		{"only dangerous chars", "<>\"'&", false, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeHostname(tt.hostname)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeHostname(%q) error = %v, wantErr %v", tt.hostname, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("SanitizeHostname(%q) = %q, want %q", tt.hostname, result, tt.expected)
			}
		})
	}
}

func TestSanitizeDisplayString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal text", "Hello World", "Hello World"},
		{"control chars", "Hello\x00World", "HelloWorld"},
		{"format string", "Hello %s World", "Hello %%s World"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeDisplayString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeDisplayString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateSettings(t *testing.T) {
	t.Run("valid settings", func(t *testing.T) {
		s := Settings{
			HomeSSID:       "MyWiFi",
			PhoneIP:        "192.168.1.100",
			PhoneMAC:       "aa:bb:cc:dd:ee:ff",
			DetectionType:  DetectionTypeMAC,
			GraceChecks:    5,
			PollInterval:   10,
			ShutdownDelay:  10,
			ShutdownAction: ShutdownActionShutdown,
			ShutdownPIN:    "1234",
		}
		warnings := ValidateSettings(&s)
		if len(warnings) != 0 {
			t.Errorf("Expected no warnings for valid settings, got %v", warnings)
		}
		// MAC should be normalized
		if s.PhoneMAC != "aa-bb-cc-dd-ee-ff" {
			t.Errorf("PhoneMAC not normalized: %q", s.PhoneMAC)
		}
	})

	t.Run("invalid SSID", func(t *testing.T) {
		s := Settings{
			HomeSSID:       "<script>alert(1)</script>",
			DetectionType:  DetectionTypeIP,
			GraceChecks:    5,
			PollInterval:   10,
			ShutdownDelay:  10,
			ShutdownAction: ShutdownActionShutdown,
		}
		warnings := ValidateSettings(&s)
		if len(warnings) == 0 {
			t.Error("Expected warnings for invalid SSID")
		}
		if s.HomeSSID != "" {
			t.Errorf("Invalid SSID should be reset to empty, got %q", s.HomeSSID)
		}
	})

	t.Run("invalid IP", func(t *testing.T) {
		s := Settings{
			PhoneIP:        "not-an-ip",
			DetectionType:  DetectionTypeIP,
			GraceChecks:    5,
			PollInterval:   10,
			ShutdownDelay:  10,
			ShutdownAction: ShutdownActionShutdown,
		}
		warnings := ValidateSettings(&s)
		if len(warnings) == 0 {
			t.Error("Expected warnings for invalid IP")
		}
		if s.PhoneIP != "" {
			t.Errorf("Invalid IP should be reset to empty, got %q", s.PhoneIP)
		}
	})

	t.Run("invalid MAC", func(t *testing.T) {
		s := Settings{
			PhoneMAC:       "not-a-mac",
			DetectionType:  DetectionTypeMAC,
			GraceChecks:    5,
			PollInterval:   10,
			ShutdownDelay:  10,
			ShutdownAction: ShutdownActionShutdown,
		}
		warnings := ValidateSettings(&s)
		if len(warnings) == 0 {
			t.Error("Expected warnings for invalid MAC")
		}
		if s.PhoneMAC != "" {
			t.Errorf("Invalid MAC should be reset to empty, got %q", s.PhoneMAC)
		}
	})

	t.Run("invalid detection type", func(t *testing.T) {
		s := Settings{
			DetectionType:  "invalid",
			GraceChecks:    5,
			PollInterval:   10,
			ShutdownDelay:  10,
			ShutdownAction: ShutdownActionShutdown,
		}
		warnings := ValidateSettings(&s)
		if len(warnings) == 0 {
			t.Error("Expected warnings for invalid detection type")
		}
		if s.DetectionType != DefaultDetectionType {
			t.Errorf("Invalid DetectionType should be reset to default, got %q", s.DetectionType)
		}
	})

	t.Run("out of range numerics", func(t *testing.T) {
		s := Settings{
			DetectionType:  DetectionTypeIP,
			GraceChecks:    -1,
			PollInterval:   999,
			ShutdownDelay:  9999,
			ShutdownAction: ShutdownActionShutdown,
		}
		warnings := ValidateSettings(&s)
		if len(warnings) < 3 {
			t.Errorf("Expected at least 3 warnings for out-of-range numerics, got %d", len(warnings))
		}
		if s.GraceChecks != DefaultGraceChecks {
			t.Errorf("GraceChecks should be reset to default, got %d", s.GraceChecks)
		}
		if s.PollInterval != DefaultPollInterval {
			t.Errorf("PollInterval should be reset to default, got %d", s.PollInterval)
		}
		if s.ShutdownDelay != DefaultShutdownDelay {
			t.Errorf("ShutdownDelay should be reset to default, got %d", s.ShutdownDelay)
		}
	})

	t.Run("invalid shutdown action", func(t *testing.T) {
		s := Settings{
			DetectionType:  DetectionTypeIP,
			GraceChecks:    5,
			PollInterval:   10,
			ShutdownDelay:  10,
			ShutdownAction: "format_c_drive",
		}
		warnings := ValidateSettings(&s)
		if len(warnings) == 0 {
			t.Error("Expected warnings for invalid shutdown action")
		}
		if s.ShutdownAction != DefaultShutdownAction {
			t.Errorf("ShutdownAction should be reset to default, got %q", s.ShutdownAction)
		}
	})

	t.Run("invalid PIN", func(t *testing.T) {
		s := Settings{
			DetectionType:  DetectionTypeIP,
			GraceChecks:    5,
			PollInterval:   10,
			ShutdownDelay:  10,
			ShutdownAction: ShutdownActionShutdown,
			ShutdownPIN:    "abc",
			RequirePIN:     true,
		}
		warnings := ValidateSettings(&s)
		if len(warnings) == 0 {
			t.Error("Expected warnings for invalid PIN")
		}
		if s.ShutdownPIN != "" {
			t.Errorf("Invalid PIN should be reset to empty, got %q", s.ShutdownPIN)
		}
		if s.RequirePIN != false {
			t.Error("RequirePIN should be reset to false when PIN is invalid")
		}
	})
}

func TestLoadWithMaliciousSettings(t *testing.T) {
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

	// Write settings with malicious values that should be sanitized on load
	settingsPath := filepath.Join(hsDir, "settings.json")
	content := `{
		"home_ssid": "<script>alert(1)</script>",
		"phone_ip": "'; DROP TABLE users; --",
		"phone_mac": "not-a-real-mac",
		"detection_type": "evil_type",
		"grace_checks": 5,
		"poll_interval_sec": 10,
		"shutdown_delay_sec": 10,
		"shutdown_action": "rm -rf /",
		"shutdown_pin": "not-digits"
	}`
	os.WriteFile(settingsPath, []byte(content), 0644)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// All malicious values should be sanitized
	if loaded.HomeSSID != "" {
		t.Errorf("Malicious SSID should be reset, got %q", loaded.HomeSSID)
	}
	if loaded.PhoneIP != "" {
		t.Errorf("Malicious IP should be reset, got %q", loaded.PhoneIP)
	}
	if loaded.PhoneMAC != "" {
		t.Errorf("Malicious MAC should be reset, got %q", loaded.PhoneMAC)
	}
	if loaded.DetectionType != DefaultDetectionType {
		t.Errorf("Malicious DetectionType should be reset, got %q", loaded.DetectionType)
	}
	if loaded.ShutdownAction != DefaultShutdownAction {
		t.Errorf("Malicious ShutdownAction should be reset, got %q", loaded.ShutdownAction)
	}
	if loaded.ShutdownPIN != "" {
		t.Errorf("Malicious PIN should be reset, got %q", loaded.ShutdownPIN)
	}
}
