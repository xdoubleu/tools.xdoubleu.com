package kobogateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	certFileName    = "cert.pem"
	keyFileName     = "key.pem"
	trustedMarker   = ".trusted"
	certValidYears  = 10
	certFilePerm    = 0o600
	certPubOrgLabel = "kobo-gateway (self-signed, local only)"
)

// EnsureCert loads a persisted self-signed cert/key from dir, generating and
// saving a new pair on first run. The cert is scoped to the loopback
// addresses the gateway ever binds to.
//
// ponytail: 10-year validity dodges a renewal story; regenerate by deleting
// cert.pem/key.pem from dir if it ever needs rotating.
func EnsureCert(dir string) (tls.Certificate, string, error) {
	certPath := filepath.Join(dir, certFileName)
	keyPath := filepath.Join(dir, keyFileName)

	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return cert, certPath, nil
	}

	certPEM, keyPEM, err := generateCert()
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("generate cert: %w", err)
	}

	if err = os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, "", fmt.Errorf("create cert dir: %w", err)
	}
	if err = os.WriteFile(certPath, certPEM, certFilePerm); err != nil {
		return tls.Certificate{}, "", fmt.Errorf("write cert: %w", err)
	}
	if err = os.WriteFile(keyPath, keyPEM, certFilePerm); err != nil {
		return tls.Certificate{}, "", fmt.Errorf("write key: %w", err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("parse generated cert: %w", err)
	}

	return cert, certPath, nil
}

func generateCert() (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{certPubOrgLabel}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(certValidYears, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal key: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

// trustCertArgs builds the `security add-trusted-cert` invocation that adds
// certPath to the current user's login keychain as a trusted root.
func trustCertArgs(certPath string) []string {
	return []string{"add-trusted-cert", "-r", "trustRoot", "-p", "ssl", certPath}
}

// TrustCertArgsForTest exposes trustCertArgs to the _test package.
func TrustCertArgsForTest(certPath string) []string { return trustCertArgs(certPath) }

// EnsureTrusted prompts the user (via the macOS Keychain UI) to trust the
// gateway's self-signed cert, once. A marker file in dir skips the prompt on
// subsequent launches; the marker is only written after `security` succeeds,
// so a cancelled prompt retries next launch.
//
// ponytail: trusts to the login keychain (no sudo); escalate to the System
// keychain (-d -k /Library/Keychains/System.keychain) only if Safari still
// rejects the cert after this.
func EnsureTrusted(dir, certPath string, out io.Writer) error {
	markerPath := filepath.Join(dir, trustedMarker)
	if _, err := os.Stat(markerPath); err == nil {
		return nil
	}

	if testing.Testing() {
		return nil
	}

	cmd := exec.Command("security", trustCertArgs(certPath)...)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("trust cert: %w", err)
	}

	return os.WriteFile(markerPath, []byte("trusted\n"), certFilePerm)
}
