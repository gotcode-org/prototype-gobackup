package engine

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateToken creates a secure, random bearer token (e.g., gb_abc123...)
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "gb_" + hex.EncodeToString(b), nil
}

// HashToken creates a SHA-256 hash of the token for secure database storage
func HashToken(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}
