package crypto_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/crypto"
)

func testKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	sealer, err := crypto.New(testKey())
	require.NoError(t, err)

	ciphertext, err := sealer.Encrypt([]byte("super secret token"))
	require.NoError(t, err)
	assert.NotContains(t, string(ciphertext), "super secret token")

	plaintext, err := sealer.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "super secret token", string(plaintext))
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	sealer, err := crypto.New(testKey())
	require.NoError(t, err)

	ciphertext, err := sealer.Encrypt([]byte("super secret token"))
	require.NoError(t, err)
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err = sealer.Decrypt(ciphertext)
	assert.Error(t, err)
}

func TestNew_InvalidKeyLength(t *testing.T) {
	_, err := crypto.New(base64.StdEncoding.EncodeToString([]byte("too short")))
	assert.Error(t, err)
}

func TestNew_InvalidKeyEncoding(t *testing.T) {
	_, err := crypto.New("not base64!!!")
	assert.Error(t, err)
}
