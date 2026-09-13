package oauth2as_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/oauth2as"
)

func pkcs8PEM(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestLoadOrGenerateOIDCKey(t *testing.T) {
	rsaKey, rsaErr := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, rsaErr)
	ecKey, ecErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, ecErr)

	pkcs1 := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	}))

	t.Run("empty yields an ephemeral key", func(t *testing.T) {
		k, generated, err := oauth2as.LoadOrGenerateOIDCKey("")
		require.NoError(t, err)
		assert.True(t, generated)
		assert.NoError(t, k.Validate())
	})

	t.Run("PKCS#8 RSA parses", func(t *testing.T) {
		k, generated, err := oauth2as.LoadOrGenerateOIDCKey(pkcs8PEM(t, rsaKey))
		require.NoError(t, err)
		assert.False(t, generated)
		assert.Equal(t, rsaKey.N, k.N)
	})

	t.Run("PKCS#1 RSA parses", func(t *testing.T) {
		k, _, err := oauth2as.LoadOrGenerateOIDCKey(pkcs1)
		require.NoError(t, err)
		assert.Equal(t, rsaKey.N, k.N)
	})

	t.Run("not PEM is rejected", func(t *testing.T) {
		_, _, err := oauth2as.LoadOrGenerateOIDCKey("not a pem block")
		require.Error(t, err)
	})

	t.Run("PEM that is not a private key is rejected", func(t *testing.T) {
		_, _, err := oauth2as.LoadOrGenerateOIDCKey(
			"-----BEGIN CERTIFICATE-----\nZm9v\n-----END CERTIFICATE-----\n",
		)
		require.Error(t, err)
	})

	t.Run("non-RSA key is rejected", func(t *testing.T) {
		_, _, err := oauth2as.LoadOrGenerateOIDCKey(pkcs8PEM(t, ecKey))
		require.Error(t, err)
	})
}

func TestLoadOIDCKeyOrDegrade(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	rsaKey, rsaErr := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, rsaErr)

	t.Run("valid key is used as-is", func(t *testing.T) {
		k := oauth2as.LoadOIDCKeyOrDegrade(logger, pkcs8PEM(t, rsaKey))
		assert.Equal(t, rsaKey.N, k.N)
	})

	t.Run("empty key degrades to an ephemeral key", func(t *testing.T) {
		k := oauth2as.LoadOIDCKeyOrDegrade(logger, "")
		assert.NoError(t, k.Validate())
	})

	t.Run(
		"invalid PEM degrades to an ephemeral key instead of panicking",
		func(t *testing.T) {
			assert.NotPanics(t, func() {
				k := oauth2as.LoadOIDCKeyOrDegrade(logger, "not a pem block")
				assert.NoError(t, k.Validate())
			})
		},
	)

	t.Run(
		"PEM that is not an RSA key degrades to an ephemeral key",
		func(t *testing.T) {
			ecKey, ecErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, ecErr)
			assert.NotPanics(t, func() {
				k := oauth2as.LoadOIDCKeyOrDegrade(logger, pkcs8PEM(t, ecKey))
				assert.NoError(t, k.Validate())
			})
		},
	)
}

func TestOIDCKeyID_StableAndDistinct(t *testing.T) {
	a, _, err := oauth2as.LoadOrGenerateOIDCKey("")
	require.NoError(t, err)
	b, _, err := oauth2as.LoadOrGenerateOIDCKey("")
	require.NoError(t, err)

	assert.Equal(t, oauth2as.OIDCKeyID(a), oauth2as.OIDCKeyID(a))
	assert.NotEqual(t, oauth2as.OIDCKeyID(a), oauth2as.OIDCKeyID(b))
	assert.NotEmpty(t, oauth2as.OIDCKeyID(a))
}
