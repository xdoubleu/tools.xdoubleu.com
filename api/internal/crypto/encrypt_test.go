package crypto_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/crypto"
)

//nolint:gochecknoglobals //shared test fixture
var testKey = []byte(
	"01234567890123456789012345678901",
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	plaintext := "super-secret-api-key"

	encrypted, err := crypto.Encrypt(testKey, plaintext)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(encrypted, "enc:"))
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := crypto.Decrypt(testKey, encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptProducesUniqueValues(t *testing.T) {
	plaintext := "same-key"

	first, err := crypto.Encrypt(testKey, plaintext)
	require.NoError(t, err)

	second, err := crypto.Encrypt(testKey, plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	// Values without the "enc:" prefix pass through unchanged (migration compatibility).
	legacy := "old-plaintext-value"
	got, err := crypto.Decrypt(testKey, legacy)
	require.NoError(t, err)
	assert.Equal(t, legacy, got)
}

func TestDecryptEmptyString(t *testing.T) {
	got, err := crypto.Decrypt(testKey, "")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	_, err := crypto.Encrypt([]byte("tooshort"), "data")
	require.Error(t, err)
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	_, err := crypto.Decrypt(testKey, "enc:!!!not-base64!!!")
	require.Error(t, err)
}

func TestDecrypt_DataTooShort(t *testing.T) {
	// AES-GCM nonce is 12 bytes; encode fewer bytes so len(data) < nonceSize.
	short := "enc:AAAA" // base64("AAAA") decodes to 3 bytes — less than 12
	_, err := crypto.Decrypt(testKey, short)
	require.Error(t, err)
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	encrypted, err := crypto.Encrypt(testKey, "original")
	require.NoError(t, err)

	// Flip the last character to corrupt the ciphertext.
	b := []byte(encrypted)
	if b[len(b)-1] == 'A' {
		b[len(b)-1] = 'B'
	} else {
		b[len(b)-1] = 'A'
	}
	_, err = crypto.Decrypt(testKey, string(b))
	require.Error(t, err)
}
