// Package updatesig signs and verifies self-update binaries with ed25519.
// Keys and signatures travel as standard base64.
package updatesig

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// ErrBadSignature is returned when a signature doesn't verify.
var ErrBadSignature = errors.New("update signature is invalid")

// GenerateKey returns a new base64 public key and base64 private seed.
func GenerateKey() (string, string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv.Seed()), nil
}

// ParsePublicKey decodes a base64 ed25519 public key.
func ParsePublicKey(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("invalid update public key")
	}
	return ed25519.PublicKey(raw), nil
}

// Sign returns the base64 signature of data under the base64 private seed.
func Sign(seedB64 string, data []byte) (string, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seedB64))
	if err != nil || len(seed) != ed25519.SeedSize {
		return "", errors.New("invalid update signing key")
	}
	sig := ed25519.Sign(ed25519.NewKeyFromSeed(seed), data)
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify checks a base64 signature of data against pub.
func Verify(pub ed25519.PublicKey, data []byte, sigB64 string) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil || !ed25519.Verify(pub, data, sig) {
		return ErrBadSignature
	}
	return nil
}
