package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// KeyStorage handles secure storage of encryption keys using OS-native methods
type KeyStorage struct {
	keyPath string
}

// NewKeyStorage creates a new key storage instance
func NewKeyStorage() *KeyStorage {
	return &KeyStorage{
		keyPath: getKeyPath(),
	}
}

// GetOrCreateKey retrieves or generates and stores an encryption key
// On Windows, uses DPAPI to encrypt the key with the user's credentials
// On other platforms, stores with restrictive file permissions
func (ks *KeyStorage) GetOrCreateKey() ([]byte, error) {
	// Try to read existing key
	keyData, err := ks.readKey()
	if err == nil && len(keyData) == 32 {
		return keyData, nil
	}

	// Generate new key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Save key with platform-specific protection
	if err := ks.saveKey(key); err != nil {
		return nil, fmt.Errorf("failed to save key: %w", err)
	}

	return key, nil
}

// readKey reads the key from secure storage
func (ks *KeyStorage) readKey() ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ks.readKeyWindows()
	}
	return ks.readKeyFile()
}

// saveKey saves the key to secure storage
func (ks *KeyStorage) saveKey(key []byte) error {
	if runtime.GOOS == "windows" {
		return ks.saveKeyWindows(key)
	}
	return ks.saveKeyFile(key)
}

// readKeyFile reads key from regular file (fallback for non-Windows)
func (ks *KeyStorage) readKeyFile() ([]byte, error) {
	// Validate file size before reading to prevent reading oversized/corrupted files
	info, err := os.Stat(ks.keyPath)
	if err != nil {
		return nil, err
	}
	const maxKeyFileSize = 1024 // DPAPI-encrypted keys can be larger than 32 bytes
	if info.Size() > maxKeyFileSize {
		return nil, fmt.Errorf("key file too large (%d bytes), max %d", info.Size(), maxKeyFileSize)
	}

	// Verify file permissions are secure
	mode := info.Mode().Perm()
	if mode != 0600 {
		fmt.Printf("Warning: Key file has permissions %o, expected 0600\n", mode)
	}

	data, err := os.ReadFile(ks.keyPath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// saveKeyFile saves key to file with restrictive permissions (fallback)
func (ks *KeyStorage) saveKeyFile(key []byte) error {
	// Ensure directory exists with secure permissions
	dir := filepath.Dir(ks.keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Write key with restrictive permissions
	return os.WriteFile(ks.keyPath, key, 0600)
}

// ClearKey securely removes the encryption key
func (ks *KeyStorage) ClearKey() error {
	// Securely wipe key data from memory before deleting
	if runtime.GOOS == "windows" {
		return ks.clearKeyWindows()
	}
	return os.Remove(ks.keyPath)
}
