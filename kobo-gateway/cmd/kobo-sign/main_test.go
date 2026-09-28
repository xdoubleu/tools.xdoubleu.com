package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/kobo-gateway/internal/updatesig"
)

func TestKeygenThenSign(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, run([]string{"keygen"}, "", &out))

	m := regexp.MustCompile(`KOBO_GATEWAY_UPDATE_PUBKEY\): (\S+)\n.*SIGNING_KEY\): (\S+)`).
		FindStringSubmatch(out.String())
	require.Len(t, m, 3)

	bin := filepath.Join(t.TempDir(), "kobo-gateway-darwin-arm64")
	require.NoError(t, os.WriteFile(bin, []byte("binary"), 0o600))
	require.NoError(t, run([]string{"sign", bin}, m[2], &out))

	sig, err := os.ReadFile(bin + ".sig")
	require.NoError(t, err)
	pub, err := updatesig.ParsePublicKey(m[1])
	require.NoError(t, err)
	assert.NoError(t, updatesig.Verify(pub, []byte("binary"), string(sig)))
}

func TestRunErrors(t *testing.T) {
	var out bytes.Buffer
	require.Error(t, run(nil, "", &out))
	missing := filepath.Join(t.TempDir(), "missing")
	require.Error(t, run([]string{"sign", missing}, "", &out))

	bin := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.WriteFile(bin, []byte("x"), 0o600))
	require.Error(t, run([]string{"sign", bin}, "", &out), "no signing key")
}
