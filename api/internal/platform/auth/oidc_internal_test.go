package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
)

const (
	testClientID = "api-client"
	testSubject  = "zitadel-user-1"
)

// fakeIssuer is an OIDC provider that signs whatever ID token a test sets.
type fakeIssuer struct {
	ts  *httptest.Server
	key *rsa.PrivateKey

	mu    sync.Mutex
	nonce string
	claim func(nonce string) jwt.MapClaims
}

func (f *fakeIssuer) idClaims() jwt.MapClaims {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.claim(f.nonce)
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	//nolint:exhaustruct // ts, nonce and claim are set per test
	issuer := &fakeIssuer{key: key}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(
		w http.ResponseWriter, _ *http.Request,
	) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer.ts.URL,
			"authorization_endpoint": issuer.ts.URL + "/authorize",
			"token_endpoint":         issuer.ts.URL + "/token",
			"jwks_uri":               issuer.ts.URL + "/keys",
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(
				big.NewInt(int64(key.E)).Bytes(),
			),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("code") != "good-code" ||
			r.PostForm.Get("code_verifier") == "" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, issuer.idClaims())
		tok.Header["kid"] = "k1"
		signed, signErr := tok.SignedString(key)
		if signErr != nil {
			http.Error(w, signErr.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "token_type": "Bearer", "id_token": signed,
		})
	})
	issuer.ts = httptest.NewServer(mux)
	t.Cleanup(issuer.ts.Close)
	return issuer
}

func (f *fakeIssuer) provider(t *testing.T) *OIDCProvider {
	t.Helper()
	//nolint:exhaustruct // only the OIDC settings matter
	provider, err := NewOIDCProvider(config.Config{
		OIDCIssuer: f.ts.URL, OIDCClientID: testClientID,
		OIDCClientSecret: "secret", APIURL: "http://api.test",
		JWTSecret: "state-signing-secret",
	})
	require.NoError(t, err)
	return provider
}

// validClaims is an ID token the provider accepts; tests break one field.
func (f *fakeIssuer) validClaims(nonce string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": f.ts.URL, "aud": testClientID, "sub": testSubject,
		"nonce": nonce, "exp": time.Now().Add(time.Hour).Unix(),
	}
}

// login runs Start then Complete as a browser would, with the issuer signing
// the claims mutate produces.
func (f *fakeIssuer) login(
	t *testing.T,
	provider *OIDCProvider,
	mutate func(jwt.MapClaims),
) (string, string, error) {
	t.Helper()
	authURL, cookie, err := provider.Start(context.Background(), "/books")
	require.NoError(t, err)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	query := parsed.Query()
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.NotEmpty(t, query.Get("code_challenge"))
	assert.Equal(t, testClientID, query.Get("client_id"))
	assert.Equal(t, "http://api.test"+OIDCCallbackPath, query.Get("redirect_uri"))

	f.mu.Lock()
	f.nonce = query.Get("nonce")
	f.claim = func(nonce string) jwt.MapClaims {
		claims := f.validClaims(nonce)
		if mutate != nil {
			mutate(claims)
		}
		return claims
	}
	f.mu.Unlock()

	callback := httptest.NewRequest(
		http.MethodGet,
		OIDCCallbackPath+"?code=good-code&state="+url.QueryEscape(query.Get("state")),
		nil,
	)
	callback.AddCookie(cookie)
	return provider.Complete(callback)
}

func TestOIDCComplete_Valid(t *testing.T) {
	issuer := newFakeIssuer(t)
	subject, returnTo, err := issuer.login(t, issuer.provider(t), nil)
	require.NoError(t, err)
	assert.Equal(t, testSubject, subject)
	assert.Equal(t, "/books", returnTo)
}

func TestOIDCComplete_Rejected(t *testing.T) {
	cases := map[string]func(jwt.MapClaims){
		"expired": func(c jwt.MapClaims) {
			c["exp"] = time.Now().Add(-time.Hour).Unix()
		},
		"wrong audience": func(c jwt.MapClaims) { c["aud"] = "someone-else" },
		"wrong issuer":   func(c jwt.MapClaims) { c["iss"] = "https://evil.test" },
		"wrong nonce":    func(c jwt.MapClaims) { c["nonce"] = "replayed" },
		"no subject":     func(c jwt.MapClaims) { delete(c, "sub") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			issuer := newFakeIssuer(t)
			subject, _, err := issuer.login(t, issuer.provider(t), mutate)
			require.Error(t, err)
			assert.Empty(t, subject)
		})
	}
}

func TestOIDCComplete_BadStateOrCode(t *testing.T) {
	issuer := newFakeIssuer(t)
	provider := issuer.provider(t)
	_, cookie, err := provider.Start(context.Background(), "/")
	require.NoError(t, err)

	for name, target := range map[string]string{
		"state mismatch": OIDCCallbackPath + "?code=good-code&state=other",
		"provider error": OIDCCallbackPath + "?error=access_denied&state=x",
		"no code":        OIDCCallbackPath + "?state=x",
		"rejected code":  OIDCCallbackPath + "?code=bad-code&state=x",
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.AddCookie(cookie)
			_, _, completeErr := provider.Complete(req)
			require.Error(t, completeErr)
		})
	}

	t.Run("no state cookie", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodGet, OIDCCallbackPath+"?code=good-code&state=x", nil,
		)
		_, _, completeErr := provider.Complete(req)
		require.Error(t, completeErr)
	})
}

func TestNewOIDCProvider_RequiresSettings(t *testing.T) {
	//nolint:exhaustruct // nothing configured
	_, err := NewOIDCProvider(config.Config{JWTSecret: "s"})
	require.Error(t, err)
}

// linkedStore knows one linked subject.
type linkedStore struct {
	usersStore
	refreshIssued bool
}

func (s *linkedStore) GetUserByOIDCSubject(
	_ context.Context, subject string,
) (*User, error) {
	if subject != testSubject {
		return nil, errNotFound
	}
	//nolint:exhaustruct // only the ID is read
	return &User{ID: "user-1"}, nil
}

func (s *linkedStore) CreateRefreshToken(
	context.Context, string, string, string, time.Time,
) error {
	s.refreshIssued = true
	return nil
}

func newLinkedService(store usersStore) *LocalService {
	//nolint:exhaustruct // only token minting is exercised
	return &LocalService{
		usersStore: store, jwtSecret: []byte("secret"),
		accessExpiry: "1h", refreshExpiry: "7d", webURL: "http://web.test",
	}
}

func TestSignInWithOIDC(t *testing.T) {
	store := &linkedStore{usersStore: nil, refreshIssued: false}
	service := newLinkedService(store)

	access, refresh, err := service.SignInWithOIDC(context.Background(), testSubject)
	require.NoError(t, err)
	assert.NotEmpty(t, *refresh)
	assert.True(t, store.refreshIssued)
	claims, err := service.parseAccessToken(*access)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.Subject)
	assert.Equal(t, aal2, claims.AAL)
}

func TestSignInWithOIDC_UnknownSubjectIsRefused(t *testing.T) {
	store := &linkedStore{usersStore: nil, refreshIssued: false}
	service := newLinkedService(store)

	access, refresh, err := service.SignInWithOIDC(context.Background(), "stranger")
	require.Error(t, err)
	assert.Nil(t, access)
	assert.Nil(t, refresh)
	assert.False(t, store.refreshIssued)

	_, _, err = service.SignInWithOIDC(context.Background(), "")
	require.Error(t, err)
}

func TestOIDCHandlers_SignInRedirectsAndSetsSession(t *testing.T) {
	issuer := newFakeIssuer(t)
	provider := issuer.provider(t)
	service := newLinkedService(&linkedStore{usersStore: nil, refreshIssued: false})

	// Login redirects to the provider and sets the state cookie; an off-site
	// return path falls back to the web home.
	rec := httptest.NewRecorder()
	OIDCLoginHandler(provider)(rec, httptest.NewRequest(
		http.MethodGet, OIDCLoginPath+"?redirect=//evil.test", nil,
	))
	require.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), issuer.ts.URL+"/authorize")
	stateCookie := rec.Result().Cookies()[0]
	assert.Equal(t, oidcStateCookie, stateCookie.Name)

	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	issuer.mu.Lock()
	issuer.nonce = location.Query().Get("nonce")
	issuer.claim = issuer.validClaims
	issuer.mu.Unlock()

	callback := httptest.NewRequest(
		http.MethodGet,
		OIDCCallbackPath+"?code=good-code&state="+location.Query().Get("state"),
		nil,
	)
	callback.AddCookie(stateCookie)
	rec = httptest.NewRecorder()
	service.OIDCCallbackHandler(provider, "http://api.test")(rec, callback)

	require.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "http://web.test/", rec.Header().Get("Location"))
	names := []string{}
	for _, c := range rec.Result().Cookies() {
		names = append(names, c.Name)
	}
	assert.Contains(t, names, "accessToken")
	assert.Contains(t, names, "refreshToken")
}

func TestOIDCCallbackHandler_UnlinkedSubjectGoesBackToSignIn(t *testing.T) {
	issuer := newFakeIssuer(t)
	provider := issuer.provider(t)
	service := newLinkedService(&linkedStore{usersStore: nil, refreshIssued: false})

	authURL, cookie, err := provider.Start(context.Background(), "/")
	require.NoError(t, err)
	location, err := url.Parse(authURL)
	require.NoError(t, err)
	issuer.mu.Lock()
	issuer.nonce = location.Query().Get("nonce")
	issuer.claim = func(nonce string) jwt.MapClaims {
		claims := issuer.validClaims(nonce)
		claims["sub"] = "stranger"
		return claims
	}
	issuer.mu.Unlock()

	callback := httptest.NewRequest(
		http.MethodGet,
		OIDCCallbackPath+"?code=good-code&state="+location.Query().Get("state"),
		nil,
	)
	callback.AddCookie(cookie)
	rec := httptest.NewRecorder()
	service.OIDCCallbackHandler(provider, "http://api.test")(rec, callback)

	assert.Equal(
		t, "http://web.test/auth/sign-in?error=oidc", rec.Header().Get("Location"),
	)
	assert.Empty(t, rec.Result().Cookies())
}

func TestOIDCCallbackHandler_ReturnsToTheAuthorizeRequest(t *testing.T) {
	issuer := newFakeIssuer(t)
	provider := issuer.provider(t)
	service := newLinkedService(&linkedStore{usersStore: nil, refreshIssued: false})

	returnTo := "/oauth2/authorize?client_id=c&state=s"
	authURL, cookie, err := provider.Start(context.Background(), returnTo)
	require.NoError(t, err)
	location, err := url.Parse(authURL)
	require.NoError(t, err)
	issuer.mu.Lock()
	issuer.nonce = location.Query().Get("nonce")
	issuer.claim = issuer.validClaims
	issuer.mu.Unlock()

	callback := httptest.NewRequest(
		http.MethodGet,
		OIDCCallbackPath+"?code=good-code&state="+location.Query().Get("state"),
		nil,
	)
	callback.AddCookie(cookie)
	rec := httptest.NewRecorder()
	service.OIDCCallbackHandler(provider, "http://api.test/api")(rec, callback)

	assert.Equal(t, "http://api.test/api"+returnTo, rec.Header().Get("Location"))
}

func TestOIDCProvider_SignInURL(t *testing.T) {
	provider := newFakeIssuer(t).provider(t)
	assert.Equal(
		t,
		OIDCLoginPath+"?redirect=%2Foauth2%2Fauthorize%3Fa%3Db",
		provider.SignInURL("/oauth2/authorize?a=b"),
	)
}

func TestOIDCLoginHandler_IssuerDown(t *testing.T) {
	issuer := newFakeIssuer(t)
	provider := issuer.provider(t)
	issuer.ts.Close()

	rec := httptest.NewRecorder()
	OIDCLoginHandler(provider)(rec, httptest.NewRequest(
		http.MethodGet, OIDCLoginPath+"?redirect=/books", nil,
	))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestOIDCCallbackHandler_ProviderErrorGoesBackToSignIn(t *testing.T) {
	provider := newFakeIssuer(t).provider(t)
	service := newLinkedService(&linkedStore{usersStore: nil, refreshIssued: false})

	rec := httptest.NewRecorder()
	service.OIDCCallbackHandler(provider, "http://api.test")(
		rec,
		httptest.NewRequest(http.MethodGet, OIDCCallbackPath+"?error=access_denied", nil),
	)
	assert.Equal(
		t, "http://web.test/auth/sign-in?error=oidc", rec.Header().Get("Location"),
	)
}

func TestIsLocalPath(t *testing.T) {
	assert.True(t, isLocalPath("/books"))
	assert.False(t, isLocalPath(""))
	assert.False(t, isLocalPath("//evil.test"))
	assert.False(t, isLocalPath("/\\evil.test"))
	assert.False(t, isLocalPath("https://evil.test"))
}

func TestECKey(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	enc := base64.RawURLEncoding.EncodeToString

	key, ok := ecKey("P-256", enc(priv.X.Bytes()), enc(priv.Y.Bytes()))
	require.True(t, ok)
	assert.True(t, key.Equal(&priv.PublicKey))

	_, ok = ecKey("P-384", enc(priv.X.Bytes()), enc(priv.Y.Bytes()))
	assert.False(t, ok)
	_, ok = ecKey("P-256", "!", "!")
	assert.False(t, ok)
	_, ok = ecKey("P-256", enc([]byte{1}), enc([]byte{1}))
	assert.False(t, ok, "a point off the curve")
}
