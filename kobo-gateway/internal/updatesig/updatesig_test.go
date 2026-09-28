package updatesig_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/kobo-gateway/internal/updatesig"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	pubB64, seedB64, err := updatesig.GenerateKey()
	require.NoError(t, err)

	pub, err := updatesig.ParsePublicKey(pubB64 + "\n")
	require.NoError(t, err)

	sig, err := updatesig.Sign(seedB64, []byte("binary"))
	require.NoError(t, err)

	require.NoError(t, updatesig.Verify(pub, []byte("binary"), sig+"\n"))
	require.ErrorIs(t, updatesig.Verify(pub, []byte("tampered"), sig),
		updatesig.ErrBadSignature)
	require.ErrorIs(t, updatesig.Verify(pub, []byte("binary"), "!!"),
		updatesig.ErrBadSignature)
}

func TestSignRejectsWrongKey(t *testing.T) {
	pubB64, _, err := updatesig.GenerateKey()
	require.NoError(t, err)
	_, otherSeed, err := updatesig.GenerateKey()
	require.NoError(t, err)

	pub, err := updatesig.ParsePublicKey(pubB64)
	require.NoError(t, err)
	sig, err := updatesig.Sign(otherSeed, []byte("binary"))
	require.NoError(t, err)

	assert.ErrorIs(t, updatesig.Verify(pub, []byte("binary"), sig),
		updatesig.ErrBadSignature)
}

func TestInvalidKeys(t *testing.T) {
	_, err := updatesig.ParsePublicKey("")
	require.Error(t, err)
	_, err = updatesig.ParsePublicKey("c2hvcnQ=")
	require.Error(t, err)

	_, err = updatesig.Sign("", []byte("x"))
	require.Error(t, err)
	_, err = updatesig.Sign("not base64!", []byte("x"))
	require.Error(t, err)
}
