package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HMAC computes an HMAC-SHA256 of the data using the provided key,
// and returns the result as a hexadecimal string.
func HMAC(data, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// LoadSelfUpdateKey loads the key used for self-update verification.
func LoadSelfUpdateKey() ([]byte, error) {
	// In the actual implementation, this would load from keyring or similar
	// For now, return a placeholder key
	return []byte("hytale-launcher-self-update-key"), nil
}
