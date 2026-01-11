package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"hytale-launcher/internal/keyring"
)

// selfUpdateKeyID is the UUID used to identify the self-update key in the keyring.
const selfUpdateKeyID = "3BA63AC3-1B08-425B-AC1A-3B19841B660D"

// HMAC computes an HMAC-SHA256 of the data using the provided key,
// and returns the result as a hexadecimal string.
func HMAC(data, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// LoadSelfUpdateKey loads the key used for self-update verification from the keyring.
func LoadSelfUpdateKey() ([]byte, error) {
	return keyring.GetOrGenKey(selfUpdateKeyID)
}
