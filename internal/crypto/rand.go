// Package crypto provides cryptographically secure random values.
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
)

// NewID returns a random 16-byte session identifier encoded as base64url.
func NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewToken returns a random 32-byte pairing token encoded as base64url
// (43 characters). It is single-use and short-lived.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewPairCode returns a 6-digit numeric pairing code, e.g. "472913".
func NewPairCode() (string, error) {
	const max = 1000000 // 10^6
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
