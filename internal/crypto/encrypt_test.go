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
