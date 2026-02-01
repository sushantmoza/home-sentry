package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// encryptString encrypts a string using AES-GCM
func encryptString(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptString decrypts a string using AES-GCM
func decryptString(ciphertext string, key []byte) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// getOrCreateKey gets or creates an encryption key stored in the user's profile
func getOrCreateKey() ([]byte, error) {
	keyPath := getKeyPath()

	// Try to read existing key
	keyData, err := os.ReadFile(keyPath)
	if err == nil && len(keyData) == 32 {
		return keyData, nil
	}

	// Generate new key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Save key
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to save key: %w", err)
	}

	return key, nil
}

// getKeyPath returns the path to the encryption key
func getKeyPath() string {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("APPDATA")
		if configDir == "" {
			configDir = os.Getenv("LOCALAPPDATA")
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, "Library", "Application Support")
	default: // Linux and others
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}

	if configDir == "" {
		configDir = "."
	}

	appDir := filepath.Join(configDir, "HomeSentry")
	os.MkdirAll(appDir, 0700)

	return filepath.Join(appDir, ".key")
}

// SecureString represents an encrypted string value
type SecureString struct {
	encrypted string
	plaintext string
	key       []byte
}

// NewSecureString creates a new SecureString from plaintext
func NewSecureString(plaintext string) (*SecureString, error) {
	key, err := getOrCreateKey()
	if err != nil {
		return nil, err
	}

	encrypted, err := encryptString(plaintext, key)
	if err != nil {
		return nil, err
	}

	return &SecureString{
		encrypted: encrypted,
		plaintext: plaintext,
		key:       key,
	}, nil
}

// NewSecureStringFromEncrypted creates a SecureString from an encrypted value
func NewSecureStringFromEncrypted(encrypted string) (*SecureString, error) {
	key, err := getOrCreateKey()
	if err != nil {
		return nil, err
	}

	plaintext, err := decryptString(encrypted, key)
	if err != nil {
		return nil, err
	}

	return &SecureString{
		encrypted: encrypted,
		plaintext: plaintext,
		key:       key,
	}, nil
}

// String returns the plaintext value (for internal use only)
func (s *SecureString) String() string {
	return s.plaintext
}

// Encrypted returns the encrypted value for storage
func (s *SecureString) Encrypted() string {
	return s.encrypted
}

// IsEmpty returns true if the string is empty
func (s *SecureString) IsEmpty() bool {
	return s.plaintext == ""
}

// Equals compares two secure strings in constant time
func (s *SecureString) Equals(other *SecureString) bool {
	if s == nil || other == nil {
		return s == other
	}
	return subtle.ConstantTimeCompare([]byte(s.plaintext), []byte(other.plaintext)) == 1
}

// EncryptSettings encrypts sensitive fields in Settings
func EncryptSettings(settings *Settings) (*Settings, error) {
	key, err := getOrCreateKey()
	if err != nil {
		return nil, err
	}

	encrypted := *settings

	// Encrypt HomeSSID
	if settings.HomeSSID != "" {
		enc, err := encryptString(settings.HomeSSID, key)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt HomeSSID: %w", err)
		}
		encrypted.HomeSSID = enc
	}

	// Encrypt PhoneMAC
	if settings.PhoneMAC != "" {
		enc, err := encryptString(settings.PhoneMAC, key)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt PhoneMAC: %w", err)
		}
		encrypted.PhoneMAC = enc
	}

	// Encrypt PhoneIP
	if settings.PhoneIP != "" {
		enc, err := encryptString(settings.PhoneIP, key)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt PhoneIP: %w", err)
		}
		encrypted.PhoneIP = enc
	}

	// Encrypt ShutdownPIN
	if settings.ShutdownPIN != "" {
		enc, err := encryptString(settings.ShutdownPIN, key)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt ShutdownPIN: %w", err)
		}
		encrypted.ShutdownPIN = enc
	}

	// Encrypt NtfyTopic (could contain sensitive info)
	if settings.NtfyTopic != "" {
		enc, err := encryptString(settings.NtfyTopic, key)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt NtfyTopic: %w", err)
		}
		encrypted.NtfyTopic = enc
	}

	return &encrypted, nil
}

// DecryptSettings decrypts sensitive fields in Settings
func DecryptSettings(settings *Settings) (*Settings, error) {
	key, err := getOrCreateKey()
	if err != nil {
		return nil, err
	}

	decrypted := *settings

	// Decrypt HomeSSID
	if settings.HomeSSID != "" {
		dec, err := decryptString(settings.HomeSSID, key)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt HomeSSID: %w", err)
		}
		decrypted.HomeSSID = dec
	}

	// Decrypt PhoneMAC
	if settings.PhoneMAC != "" {
		dec, err := decryptString(settings.PhoneMAC, key)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt PhoneMAC: %w", err)
		}
		decrypted.PhoneMAC = dec
	}

	// Decrypt PhoneIP
	if settings.PhoneIP != "" {
		dec, err := decryptString(settings.PhoneIP, key)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt PhoneIP: %w", err)
		}
		decrypted.PhoneIP = dec
	}

	// Decrypt ShutdownPIN
	if settings.ShutdownPIN != "" {
		dec, err := decryptString(settings.ShutdownPIN, key)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt ShutdownPIN: %w", err)
		}
		decrypted.ShutdownPIN = dec
	}

	// Decrypt NtfyTopic
	if settings.NtfyTopic != "" {
		dec, err := decryptString(settings.NtfyTopic, key)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt NtfyTopic: %w", err)
		}
		decrypted.NtfyTopic = dec
	}

	return &decrypted, nil
}
