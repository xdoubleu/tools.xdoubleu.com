package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/errortools"
)

// Routes people sign in through when OIDC sign-in is on, and the cookie that
// carries the login's state between them.
const (
	OIDCLoginPath    = "/auth/oidc/login"
	OIDCCallbackPath = "/auth/oidc/callback"

	oidcStateCookie = "oidcState"
	oidcStateTTL    = 10 * time.Minute
	oidcRandBytes   = 16
	oidcCacheTTL    = 10 * time.Minute
	oidcHTTPTimeout = 10 * time.Second
	oidcLeeway      = 30 * time.Second

	// authorizePrefix marks a return path served by this api rather than the web.
	authorizePrefix = "/oauth2/"
)

var errOIDCSignIn = errortools.NewUnauthorizedError(errors.New("oidc sign-in failed"))

// OIDCProvider is the api as an OIDC relying party for people: authorization
// code with PKCE against a configured issuer, Zitadel by default.
type OIDCProvider struct {
	issuer      string
	clientID    string
	secret      string
	redirectURL string
	stateKey    []byte
	secure      bool
	client      *http.Client

	mu        sync.Mutex
	disc      *oidcDiscovery
	discAt    time.Time
	keys      map[string]crypto.PublicKey
	keysAt    time.Time
	nowFunc   func() time.Time
	keyReload time.Time
}

type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// NewOIDCProvider configures the provider from cfg; the issuer is only
// contacted on first use.
func NewOIDCProvider(cfg config.Config) (*OIDCProvider, error) {
	if cfg.OIDCIssuer == "" || cfg.OIDCClientID == "" || cfg.OIDCClientSecret == "" {
		return nil, errors.New(
			"auth: OIDC_ISSUER, OIDC_CLIENT_ID and OIDC_CLIENT_SECRET must be set " +
				"when OIDC_SIGN_IN is on",
		)
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("auth: JWT_SECRET must be set when OIDC_SIGN_IN is on")
	}
	//nolint:exhaustruct // the caches fill on first use
	return &OIDCProvider{
		issuer:      strings.TrimSuffix(cfg.OIDCIssuer, "/"),
		clientID:    cfg.OIDCClientID,
		secret:      cfg.OIDCClientSecret,
		redirectURL: strings.TrimSuffix(cfg.APIURL, "/") + OIDCCallbackPath,
		stateKey:    []byte(cfg.JWTSecret),
		secure:      cfg.Env == config.ProdEnv,
		client:      &http.Client{Timeout: oidcHTTPTimeout},
		nowFunc:     time.Now,
	}, nil
}

// SignInURL is where to send a person to sign in, returning to returnTo.
func (p *OIDCProvider) SignInURL(returnTo string) string {
	return OIDCLoginPath + "?redirect=" + url.QueryEscape(returnTo)
}

// stateClaims is what the state cookie carries from login to callback.
type stateClaims struct {
	jwt.RegisteredClaims
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	ReturnTo string `json:"return_to"`
}

func randomString() (string, error) {
	buf := make([]byte, oidcRandBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (p *OIDCProvider) oauth2Config(disc *oidcDiscovery) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.secret,
		RedirectURL:  p.redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:       disc.AuthorizationEndpoint,
			TokenURL:      disc.TokenEndpoint,
			DeviceAuthURL: "",
			AuthStyle:     oauth2.AuthStyleAutoDetect,
		},
	}
}

// Start begins a login: the provider URL to redirect to and the cookie that
// binds the callback to this browser.
func (p *OIDCProvider) Start(
	ctx context.Context,
	returnTo string,
) (string, *http.Cookie, error) {
	disc, err := p.discovery(ctx)
	if err != nil {
		return "", nil, err
	}
	state, err := randomString()
	if err != nil {
		return "", nil, err
	}
	nonce, err := randomString()
	if err != nil {
		return "", nil, err
	}
	verifier := oauth2.GenerateVerifier()

	now := p.nowFunc()
	//nolint:exhaustruct // other RegisteredClaims fields are optional
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, stateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(oidcStateTTL)),
		},
		State: state, Nonce: nonce, Verifier: verifier, ReturnTo: returnTo,
	}).SignedString(p.stateKey)
	if err != nil {
		return "", nil, err
	}

	authURL := p.oauth2Config(disc).AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("nonce", nonce),
	)
	//nolint:gosec // Secure follows the environment
	cookie := &http.Cookie{
		Name:     oidcStateCookie,
		Value:    signed,
		MaxAge:   int(oidcStateTTL.Seconds()),
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   p.secure,
		Path:     "/",
	}
	return authURL, cookie, nil
}

// Complete finishes a login from the provider's redirect: it checks the state,
// exchanges the code and validates the ID token. It returns the provider
// subject and where the person was headed.
func (p *OIDCProvider) Complete(
	r *http.Request,
) (string, string, error) {
	ctx := r.Context()
	query := r.URL.Query()
	if query.Get("error") != "" || query.Get("code") == "" {
		return "", "", errOIDCSignIn
	}

	cookie, err := r.Cookie(oidcStateCookie)
	if err != nil {
		return "", "", errOIDCSignIn
	}
	var sc stateClaims
	token, err := jwt.ParseWithClaims(
		cookie.Value, &sc,
		func(_ *jwt.Token) (any, error) { return p.stateKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid || sc.State == "" || sc.State != query.Get("state") {
		return "", "", errOIDCSignIn
	}

	disc, err := p.discovery(ctx)
	if err != nil {
		return "", "", err
	}
	tok, err := p.oauth2Config(disc).Exchange(
		context.WithValue(ctx, oauth2.HTTPClient, p.client),
		query.Get("code"),
		oauth2.VerifierOption(sc.Verifier),
	)
	if err != nil {
		return "", "", errOIDCSignIn
	}
	rawID, _ := tok.Extra("id_token").(string)
	subject, err := p.verifyIDToken(ctx, disc, rawID, sc.Nonce)
	if err != nil {
		return "", "", err
	}
	return subject, sc.ReturnTo, nil
}

type idClaims struct {
	jwt.RegisteredClaims
	Nonce string `json:"nonce"`
}

// verifyIDToken checks the signature against the issuer's keys, the issuer,
// audience, expiry and nonce, and returns the subject.
func (p *OIDCProvider) verifyIDToken(
	ctx context.Context,
	disc *oidcDiscovery,
	raw, nonce string,
) (string, error) {
	if raw == "" {
		return "", errOIDCSignIn
	}
	var c idClaims
	token, err := jwt.ParseWithClaims(
		raw, &c,
		func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			return p.signingKey(ctx, disc, kid)
		},
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
		jwt.WithIssuer(disc.Issuer),
		jwt.WithAudience(p.clientID),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(oidcLeeway),
		jwt.WithTimeFunc(p.nowFunc),
	)
	if err != nil || !token.Valid {
		return "", errOIDCSignIn
	}
	if c.Subject == "" || nonce == "" || c.Nonce != nonce {
		return "", errOIDCSignIn
	}
	return c.Subject, nil
}

func (p *OIDCProvider) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: oidc GET %s: status %d", endpoint, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// discovery returns the issuer's endpoints, cached for oidcCacheTTL.
func (p *OIDCProvider) discovery(ctx context.Context) (*oidcDiscovery, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.disc != nil && p.nowFunc().Sub(p.discAt) < oidcCacheTTL {
		return p.disc, nil
	}
	var disc oidcDiscovery
	if err := p.getJSON(
		ctx, p.issuer+"/.well-known/openid-configuration", &disc,
	); err != nil {
		return nil, err
	}
	if strings.TrimSuffix(disc.Issuer, "/") != p.issuer ||
		disc.AuthorizationEndpoint == "" || disc.TokenEndpoint == "" ||
		disc.JWKSURI == "" {
		return nil, errors.New("auth: oidc discovery document doesn't match the issuer")
	}
	p.disc, p.discAt = &disc, p.nowFunc()
	return p.disc, nil
}

// signingKey returns the issuer's key for kid, reloading the key set once
// when the kid is unknown (rotation), at most once per minute.
func (p *OIDCProvider) signingKey(
	ctx context.Context,
	disc *oidcDiscovery,
	kid string,
) (crypto.PublicKey, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.nowFunc()
	if key, ok := p.lookupKey(kid); ok && now.Sub(p.keysAt) < oidcCacheTTL {
		return key, nil
	}
	if p.keys != nil && now.Sub(p.keyReload) < time.Minute {
		return nil, errors.New("auth: unknown oidc signing key")
	}
	p.keyReload = now

	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	if err := p.getJSON(ctx, disc.JWKSURI, &set); err != nil {
		return nil, err
	}
	keys := map[string]crypto.PublicKey{}
	for _, k := range set.Keys {
		switch k.Kty {
		case "RSA":
			n, e, ok := decodeInts(k.N, k.E)
			if ok && e.IsInt64() {
				keys[k.Kid] = &rsa.PublicKey{N: n, E: int(e.Int64())}
			}
		case "EC":
			if key, ok := ecKey(k.Crv, k.X, k.Y); ok {
				keys[k.Kid] = key
			}
		}
	}
	p.keys, p.keysAt = keys, now
	if key, ok := p.lookupKey(kid); ok {
		return key, nil
	}
	return nil, errors.New("auth: unknown oidc signing key")
}

// lookupKey finds kid, or the only key when the token names none.
func (p *OIDCProvider) lookupKey(kid string) (crypto.PublicKey, bool) {
	if key, ok := p.keys[kid]; ok {
		return key, true
	}
	if kid == "" && len(p.keys) == 1 {
		for _, key := range p.keys {
			return key, true
		}
	}
	return nil, false
}

func decodeInts(a, b string) (*big.Int, *big.Int, bool) {
	ab, errA := base64.RawURLEncoding.DecodeString(a)
	bb, errB := base64.RawURLEncoding.DecodeString(b)
	if errA != nil || errB != nil {
		return nil, nil, false
	}
	return new(big.Int).SetBytes(ab), new(big.Int).SetBytes(bb), true
}

// ecKey builds a P-256 key from its JWK coordinates.
func ecKey(curve, x, y string) (*ecdsa.PublicKey, bool) {
	xb, errX := base64.RawURLEncoding.DecodeString(x)
	yb, errY := base64.RawURLEncoding.DecodeString(y)
	const coordLen = 32
	if curve != "P-256" || errX != nil || errY != nil ||
		len(xb) > coordLen || len(yb) > coordLen {
		return nil, false
	}
	point := make([]byte, 1+2*coordLen)
	point[0] = 4 // uncompressed
	copy(point[1+coordLen-len(xb):], xb)
	copy(point[1+2*coordLen-len(yb):], yb)
	key, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
	return key, err == nil
}
