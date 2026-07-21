// Package crypto encrypts small secrets (OAuth tokens) at rest with
// AES-256-GCM.
//
// ponytail: one static key, no rotation/versioning — add a key-id prefix to
// the ciphertext if rotation is ever needed.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const keyLen = 32 // AES-256

// Sealer encrypts/decrypts secrets with a single AES-256-GCM key.
type Sealer struct {
	gcm cipher.AEAD
}

// New builds a Sealer from a base64-standard-encoded 32-byte key (e.g.
// OAUTH_TOKEN_ENC_KEY). Generate one with `openssl rand -base64 32`.
func New(keyB64 string) (*Sealer, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("crypto: invalid key encoding: %w", err)
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf(
			"crypto: key must be %d raw bytes, got %d", keyLen, len(key),
		)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Sealer{gcm: gcm}, nil
}

// Encrypt seals plaintext, prepending a random nonce to the returned ciphertext.
func (s *Sealer) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens ciphertext produced by Encrypt.
func (s *Sealer) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := s.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return s.gcm.Open(nil, nonce, sealed, nil)
}
