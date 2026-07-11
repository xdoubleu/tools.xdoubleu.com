package kobogateway_test

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/kobogateway"
)

func TestEnsureCertGeneratesAndReuses(t *testing.T) {
	dir := t.TempDir()

	cert, certPath, err := kobogateway.EnsureCert(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "cert.pem"), certPath)
	require.NotEmpty(t, cert.Certificate)

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	assert.Contains(t, leaf.DNSNames, "localhost")
	require.Len(t, leaf.IPAddresses, 1)
	assert.Equal(t, "127.0.0.1", leaf.IPAddresses[0].String())

	// Second call must reuse the persisted cert, not regenerate it.
	cert2, _, err := kobogateway.EnsureCert(dir)
	require.NoError(t, err)
	assert.Equal(t, cert.Certificate[0], cert2.Certificate[0])
}

func TestEnsureTrustedNoOpUnderTest(t *testing.T) {
	dir := t.TempDir()

	// testing.Testing() is true in the test binary, so this must never shell
	// out to `security` (which would hang/fail in CI).
	err := kobogateway.EnsureTrusted(dir, filepath.Join(dir, "cert.pem"), nil)
	assert.NoError(t, err)
}

func TestEnsureTrustedSkipsWhenMarkerExists(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".trusted"), []byte("trusted\n"), 0o600))

	// Marker already present, so this must return before even checking
	// testing.Testing().
	err := kobogateway.EnsureTrusted(dir, filepath.Join(dir, "cert.pem"), nil)
	assert.NoError(t, err)
}

func TestTrustCertArgsForTest(t *testing.T) {
	assert.Equal(t,
		[]string{"add-trusted-cert", "-r", "trustRoot", "-p", "ssl", "/tmp/cert.pem"},
		kobogateway.TrustCertArgsForTest("/tmp/cert.pem"),
	)
}
