package garden

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// HashFile computes the SHA-256 hash of the file at path and returns
// the hex-encoded hash string.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("hashing file: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
