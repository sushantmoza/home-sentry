//go:build windows

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

// DATA_BLOB structure for DPAPI
type DATA_BLOB struct {
	cbData uint32
	pbData *byte
}

// readKeyWindows reads and decrypts the key using Windows DPAPI
func (ks *KeyStorage) readKeyWindows() ([]byte, error) {
	encryptedKey, err := os.ReadFile(ks.keyPath)
	if err != nil {
		return nil, err
	}

	// Decrypt using DPAPI
	return dpapiDecrypt(encryptedKey)
}

// saveKeyWindows encrypts and saves the key using Windows DPAPI
func (ks *KeyStorage) saveKeyWindows(key []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(ks.keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Encrypt using DPAPI
	encryptedKey, err := dpapiEncrypt(key)
	if err != nil {
		return fmt.Errorf("DPAPI encryption failed: %w", err)
	}

	return os.WriteFile(ks.keyPath, encryptedKey, 0600)
}

// clearKeyWindows securely removes the key file
func (ks *KeyStorage) clearKeyWindows() error {
	return os.Remove(ks.keyPath)
}

// dpapiEncrypt encrypts data using Windows DPAPI (CurrentUser scope)
func dpapiEncrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("nothing to encrypt")
	}

	dataIn := DATA_BLOB{
		cbData: uint32(len(plaintext)),
		pbData: &plaintext[0],
	}

	var dataOut DATA_BLOB

	// CRYPTPROTECT_LOCAL_MACHINE = 0x4 (optional, using CurrentUser by default)
	// CRYPTPROTECT_UI_FORBIDDEN = 0x1
	ret, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&dataIn)),
		0, // No description
		0, // No additional entropy
		0, // Reserved
		0, // No prompt struct
		1, // CRYPTPROTECT_UI_FORBIDDEN
		uintptr(unsafe.Pointer(&dataOut)),
	)

	if ret == 0 {
		return nil, fmt.Errorf("CryptProtectData failed: %v", err)
	}

	// Copy encrypted data
	encrypted := make([]byte, dataOut.cbData)
	if dataOut.cbData > 0 {
		copy(encrypted, (*[1 << 30]byte)(unsafe.Pointer(dataOut.pbData))[:dataOut.cbData:dataOut.cbData])
	}

	// Free memory allocated by DPAPI
	procLocalFree.Call(uintptr(unsafe.Pointer(dataOut.pbData)))

	return encrypted, nil
}

// dpapiDecrypt decrypts data using Windows DPAPI
func dpapiDecrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("nothing to decrypt")
	}

	dataIn := DATA_BLOB{
		cbData: uint32(len(ciphertext)),
		pbData: &ciphertext[0],
	}

	var dataOut DATA_BLOB

	// CRYPTUNPROTECT_UI_FORBIDDEN = 0x1
	ret, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&dataIn)),
		0, // No description
		0, // No additional entropy
		0, // Reserved
		0, // No prompt struct
		1, // CRYPTUNPROTECT_UI_FORBIDDEN
		uintptr(unsafe.Pointer(&dataOut)),
	)

	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed: %v", err)
	}

	// Copy decrypted data
	decrypted := make([]byte, dataOut.cbData)
	if dataOut.cbData > 0 {
		copy(decrypted, (*[1 << 30]byte)(unsafe.Pointer(dataOut.pbData))[:dataOut.cbData:dataOut.cbData])
	}

	// Free memory allocated by DPAPI
	procLocalFree.Call(uintptr(unsafe.Pointer(dataOut.pbData)))

	return decrypted, nil
}
