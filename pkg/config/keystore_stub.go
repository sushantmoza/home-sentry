//go:build !windows

package config

import "errors"

// Stub implementations for non-Windows platforms
// These should never be called in practice since this is a Windows app

func (ks *KeyStorage) readKeyWindows() ([]byte, error) {
	return nil, ErrNotImplemented
}

func (ks *KeyStorage) saveKeyWindows(key []byte) error {
	return ErrNotImplemented
}

func (ks *KeyStorage) clearKeyWindows() error {
	return ErrNotImplemented
}

// ErrNotImplemented indicates the operation is not available on this platform
var ErrNotImplemented = errors.New("operation not implemented on this platform")
