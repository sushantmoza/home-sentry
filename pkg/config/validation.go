package config

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// SSID can contain any characters but has length limits
	ssidRegex = regexp.MustCompile(`^[\x20-\x7E]{1,32}$`)

	// MAC address formats: 00:11:22:33:44:55 or 00-11-22-33-44-55 or 001122334455
	macRegex        = regexp.MustCompile(`^([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}$`)
	macCompactRegex = regexp.MustCompile(`^[0-9a-fA-F]{12}$`)

	// IP address validation
	ipRegex = regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)

	// PIN validation - 4-8 digits only
	pinRegex = regexp.MustCompile(`^\d{4,8}$`)

	// Topic name validation for ntfy (alphanumeric, hyphens, underscores)
	topicRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

	// General dangerous character pattern (for basic XSS prevention)
	dangerousChars = regexp.MustCompile(`[<>"'&]|javascript:|data:|vbscript:`)
)

// SanitizeSSID validates and sanitizes an SSID string
func SanitizeSSID(ssid string) (string, error) {
	// Trim whitespace
	ssid = strings.TrimSpace(ssid)

	// Check length
	if len(ssid) == 0 {
		return "", nil // Empty is valid
	}
	if len(ssid) > MaxSSIDLength {
		return "", NewValidationError("SSID too long", "SSID must be 32 characters or less")
	}

	// Check for dangerous characters
	if dangerousChars.MatchString(ssid) {
		return "", NewValidationError("SSID contains invalid characters", "SSID contains potentially dangerous characters")
	}

	return ssid, nil
}

// SanitizeMAC validates and normalizes a MAC address
func SanitizeMAC(mac string) (string, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return "", nil
	}

	// Check compact format (12 hex chars)
	if macCompactRegex.MatchString(mac) {
		// Convert to standard format with dashes
		var result strings.Builder
		for i := 0; i < 12; i += 2 {
			if i > 0 {
				result.WriteByte('-')
			}
			result.WriteString(mac[i : i+2])
		}
		return strings.ToLower(result.String()), nil
	}

	// Check standard format
	if !macRegex.MatchString(mac) {
		return "", NewValidationError("Invalid MAC address", "MAC must be in format AA:BB:CC:DD:EE:FF or AA-BB-CC-DD-EE-FF")
	}

	// Normalize to lowercase with dashes
	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, ":", "-")
	return mac, nil
}

// SanitizeIP validates and sanitizes an IP address
func SanitizeIP(ip string) (string, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", nil
	}

	if !ipRegex.MatchString(ip) {
		return "", NewValidationError("Invalid IP address", "IP must be in format xxx.xxx.xxx.xxx")
	}

	return ip, nil
}

// SanitizePIN validates a PIN
func SanitizePIN(pin string) (string, error) {
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return "", nil
	}

	if !pinRegex.MatchString(pin) {
		return "", NewValidationError("Invalid PIN", "PIN must be 4-8 digits")
	}

	return pin, nil
}

// SanitizeTopic validates and sanitizes a topic name
func SanitizeTopic(topic string) (string, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return "", nil
	}

	if !topicRegex.MatchString(topic) {
		return "", NewValidationError("Invalid topic name", "Topic must be alphanumeric with hyphens/underscores (1-64 chars)")
	}

	return strings.ToLower(topic), nil
}

// SanitizeServerURL validates a server URL (basic validation)
func SanitizeServerURL(url string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", nil
	}

	// Must start with http:// or https://
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", NewValidationError("Invalid URL", "URL must start with http:// or https://")
	}

	// Check for dangerous protocols
	if dangerousChars.MatchString(url) {
		return "", NewValidationError("URL contains invalid characters", "URL contains potentially dangerous content")
	}

	return url, nil
}

// IsPrintableASCII checks if a string contains only printable ASCII characters
func IsPrintableASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// RemoveControlChars removes control characters from a string
func RemoveControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

// ValidationError represents a validation error with user-friendly message
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
