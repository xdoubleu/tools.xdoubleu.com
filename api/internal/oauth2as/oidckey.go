package oauth2as

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
)

// oidcKeyBits is the ephemeral key size; 2048 is RS256's expected floor.
const oidcKeyBits = 2048

// LoadOrGenerateOIDCKey parses pemKey (PKCS#8 or PKCS#1) or, when empty,
// generates an ephemeral key (generated=true). Ephemeral keys invalidate ID
// tokens on restart, so production sets OAUTH_OIDC_PRIVATE_KEY.
func LoadOrGenerateOIDCKey(pemKey string) (*rsa.PrivateKey, bool, error) {
	if pemKey == "" {
		k, err := rsa.GenerateKey(rand.Reader, oidcKeyBits)
		if err != nil {
			return nil, false, err
		}
		return k, true, nil
	}

	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, false, errors.New(
			"oauth2as: OAUTH_OIDC_PRIVATE_KEY is not valid PEM",
		)
	}

	if k, pkcs1Err := x509.ParsePKCS1PrivateKey(block.Bytes); pkcs1Err == nil {
		return k, false, nil
	}

	parsed, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if pkcs8Err != nil {
		return nil, false, errors.New(
			"oauth2as: OAUTH_OIDC_PRIVATE_KEY is neither a PKCS#1 nor a PKCS#8 RSA private key",
		)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, false, errors.New(
			"oauth2as: OAUTH_OIDC_PRIVATE_KEY is not an RSA key",
		)
	}
	return rsaKey, false, nil
}

// LoadOIDCKeyOrDegrade is LoadOrGenerateOIDCKey, but a malformed key logs an
// Error and falls back to an ephemeral one so it can't block startup.
func LoadOIDCKeyOrDegrade(logger *slog.Logger, pemKey string) *rsa.PrivateKey {
	key, generated, err := LoadOrGenerateOIDCKey(pemKey)
	if err != nil {
		logger.Error(
			"OAUTH_OIDC_PRIVATE_KEY is invalid — falling back to an ephemeral "+
				"OIDC signing key; ID tokens issued before a restart will not "+
				"verify afterwards and Grafana SSO users will need to re-log-in",
			"error", err,
		)
		var genErr error
		key, _, genErr = LoadOrGenerateOIDCKey("")
		if genErr != nil {
			panic(genErr)
		}
		return key
	}
	if generated {
		logger.Warn(
			"OAUTH_OIDC_PRIVATE_KEY unset — using an ephemeral OIDC signing key; " +
				"ID tokens issued before a restart will not verify afterwards",
		)
	}
	return key
}

// OIDCKeyID derives a stable kid (base64url SHA-256 of the RFC 7638 thumbprint
// input) so tokens and JWKS agree across restarts.
func OIDCKeyID(key *rsa.PrivateKey) string {
	return oidcKeyID(&key.PublicKey)
}

func oidcKeyID(pub *rsa.PublicKey) string {
	thumbprintInput := `{"e":"` + b64u(big.NewInt(int64(pub.E)).Bytes()) +
		`","kty":"RSA","n":"` + b64u(pub.N.Bytes()) + `"}`
	sum := sha256.Sum256([]byte(thumbprintInput))
	return b64u(sum[:])
}

func b64u(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func jwks(key *rsa.PrivateKey) map[string]any {
	pub := &key.PublicKey
	return map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": oidcKeyID(pub),
			"n":   b64u(pub.N.Bytes()),
			"e":   b64u(big.NewInt(int64(pub.E)).Bytes()),
		}},
	}
}

// JWKSHandler serves the OIDC signing key's JWKS at /oauth2/jwks.
func JWKSHandler(key *rsa.PrivateKey) http.HandlerFunc {
	doc := jwks(key)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(doc)
	}
}
