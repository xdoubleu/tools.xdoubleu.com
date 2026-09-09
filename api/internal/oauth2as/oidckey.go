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
	"math/big"
	"net/http"
)

// oidcKeyBits is the RSA key size used when no OAUTH_OIDC_PRIVATE_KEY is
// configured and an ephemeral key is generated at startup instead. 2048 is
// the floor RS256 ID-token signing keys are expected to meet.
const oidcKeyBits = 2048

// LoadOrGenerateOIDCKey returns the RSA private key that signs OIDC ID tokens
// (and whose public half is published at /oauth2/jwks). A non-empty pemKey is
// parsed as a PEM-encoded PKCS#8 or PKCS#1 private key; an empty pemKey yields
// a freshly generated ephemeral key (generated is true) so local development
// and tests need no key material configured — the tradeoff being that every
// process restart invalidates ID tokens issued by the previous one, which is
// why production sets OAUTH_OIDC_PRIVATE_KEY.
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

// OIDCKeyID derives a stable, deterministic key id from the signing key, so
// the `kid` in a signed ID token's header matches the `kid` of the JWKS entry
// regardless of process restarts (as long as the key material is the same).
// It is the base64url-encoded SHA-256 of the RFC 7638 JWK thumbprint input.
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

// jwks builds the RFC 7517 JSON Web Key Set document exposing the public half
// of key for RS256 ID-token verification.
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

// JWKSHandler serves the JSON Web Key Set for the OIDC signing key at
// /oauth2/jwks, letting a relying party (Grafana) verify ID-token signatures.
func JWKSHandler(key *rsa.PrivateKey) http.HandlerFunc {
	doc := jwks(key)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(doc)
	}
}
