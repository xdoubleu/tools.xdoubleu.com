package kobogateway_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/kobo-gateway/internal/kobogateway"
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

	// The trusted root only vouches for loopback names.
	assert.True(t, leaf.PermittedDNSDomainsCritical)
	assert.Equal(t, []string{"localhost"}, leaf.PermittedDNSDomains)
	require.Len(t, leaf.PermittedIPRanges, 1)
	assert.Equal(t, "127.0.0.1/32", leaf.PermittedIPRanges[0].String())

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
	require.NoError(
		t, os.WriteFile(filepath.Join(dir, ".trusted"), []byte("trusted\n"), 0o600),
	)

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

// An older gateway's unconstrained root is replaced and must be re-trusted.
func TestEnsureCertReplacesUnconstrainedCert(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := unconstrainedCertPEM(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cert.pem"), certPEM, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "key.pem"), keyPEM, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".trusted"), nil, 0o600))

	cert, _, err := kobogateway.EnsureCert(dir)
	require.NoError(t, err)

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	assert.True(t, leaf.PermittedDNSDomainsCritical)
	assert.NoFileExists(t, filepath.Join(dir, ".trusted"))
}

func unconstrainedCertPEM(t *testing.T) ([]byte, []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	// An old-style root: CA, no name constraints.
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}
	der, err := x509.CreateCertificate(
		rand.Reader, template, template, &priv.PublicKey, priv,
	)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
