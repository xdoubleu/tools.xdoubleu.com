package githubapp_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/githubapp"
)

func newKey(t *testing.T) (*rsa.PrivateKey, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pkcs1 := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	pkcs8 := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return key, string(pkcs1), string(pkcs8)
}

// fakeGitHub answers like GitHub, checking the App JWT with the public key.
type fakeGitHub struct {
	t             *testing.T
	pub           *rsa.PublicKey
	appID         string
	installations atomic.Int32
	mints         atomic.Int32
	installStatus int
	mintStatus    int
	mintBody      string
	lastMint      map[string]any
	server        *httptest.Server
	mux           *http.ServeMux
}

func newFake(t *testing.T, key *rsa.PrivateKey) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{
		t: t, pub: &key.PublicKey, appID: "12345",
		installStatus: http.StatusOK, mintStatus: http.StatusCreated,
		installations: atomic.Int32{}, mints: atomic.Int32{}, mintBody: "",
		lastMint: nil, server: nil, mux: nil,
	}
	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /repos/{owner}/{repo}/installation",
		func(w http.ResponseWriter, r *http.Request) {
			f.installations.Add(1)
			f.checkJWT(r)
			assert.Empty(t, r.Header.Get("Content-Type"), "a GET has no body")
			assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
			if f.installStatus != http.StatusOK {
				w.WriteHeader(f.installStatus)
				return
			}
			_, _ = io.WriteString(w, `{"id": 777}`)
		},
	)
	mux.HandleFunc(
		"POST /app/installations/{id}/access_tokens",
		func(w http.ResponseWriter, r *http.Request) {
			f.mints.Add(1)
			f.checkJWT(r)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "777", r.PathValue("id"))
			body, _ := io.ReadAll(r.Body)
			require.NoError(t, json.Unmarshal(body, &f.lastMint))
			if f.mintStatus != http.StatusCreated {
				w.WriteHeader(f.mintStatus)
				_, _ = io.WriteString(w, `{"message":"secret detail"}`)
				return
			}
			w.WriteHeader(http.StatusCreated)
			body2 := f.mintBody
			if body2 == "" {
				body2 = `{"token":"ghs_minted","expires_at":"2026-11-04T10:00:00Z"}`
			}
			_, _ = io.WriteString(w, body2)
		},
	)
	f.mux = mux
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) checkJWT(r *http.Request) {
	f.t.Helper()
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	require.True(f.t, ok)
	parts := strings.Split(token, ".")
	require.Len(f.t, parts, 3)
	enc := base64.RawURLEncoding

	header, err := enc.DecodeString(parts[0])
	require.NoError(f.t, err)
	assert.JSONEq(f.t, `{"alg":"RS256","typ":"JWT"}`, string(header))

	rawClaims, err := enc.DecodeString(parts[1])
	require.NoError(f.t, err)
	var claims struct {
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
		Iss string `json:"iss"`
	}
	require.NoError(f.t, json.Unmarshal(rawClaims, &claims))
	assert.Equal(f.t, f.appID, claims.Iss)
	assert.Equal(
		f.t,
		int64(9*60+60),
		claims.Exp-claims.Iat,
		"backdated a minute, lives nine",
	)

	sig, err := enc.DecodeString(parts[2])
	require.NoError(f.t, err)
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	assert.NoError(f.t, rsa.VerifyPKCS1v15(f.pub, crypto.SHA256, sum[:], sig))
}

func newClient(t *testing.T, f *fakeGitHub, keyPEM string) *githubapp.Client {
	t.Helper()
	fixed := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	c, err := githubapp.New("12345", keyPEM,
		githubapp.WithBaseURL(f.server.URL),
		githubapp.WithHTTPClient(f.server.Client()),
		githubapp.WithClock(func() time.Time { return fixed }))
	require.NoError(t, err)
	return c
}

func TestMintsATokenNarrowedToOneRepoAndWhatAJobNeeds(t *testing.T) {
	key, pkcs1, pkcs8 := newKey(t)
	for name, pemText := range map[string]string{"PKCS#1": pkcs1, "PKCS#8": pkcs8} {
		f := newFake(t, key)
		c := newClient(t, f, pemText)

		token, err := c.InstallationToken(t.Context(), "xdoubleu", "tools.xdoubleu.com")
		require.NoError(t, err, name)
		assert.Equal(t, "ghs_minted", token.Value)
		assert.Equal(t, time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC), token.ExpiresAt)

		assert.Equal(t, []any{"tools.xdoubleu.com"}, f.lastMint["repositories"])
		assert.Equal(t, map[string]any{"contents": "write", "pull_requests": "write"},
			f.lastMint["permissions"], "no workflows, issues or administration")
	}
}

func TestTheInstallationIsRememberedUntilItGoesStale(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	c := newClient(t, f, pkcs1)

	for range 3 {
		_, err := c.InstallationToken(t.Context(), "o", "r")
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, f.installations.Load(), "looked up once")
	assert.EqualValues(t, 3, f.mints.Load())

	// The App was reinstalled: the remembered id 404s, and the next call looks again.
	f.mintStatus = http.StatusNotFound
	_, err := c.InstallationToken(t.Context(), "o", "r")
	require.ErrorIs(t, err, githubapp.ErrNotInstalled)
	f.mintStatus = http.StatusCreated
	_, err = c.InstallationToken(t.Context(), "o", "r")
	require.NoError(t, err)
	assert.EqualValues(t, 2, f.installations.Load())
}

func TestAnAppNotInstalledOnTheRepoSaysSo(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	f.installStatus = http.StatusNotFound
	c := newClient(t, f, pkcs1)
	_, err := c.InstallationToken(t.Context(), "o", "r")
	require.ErrorIs(t, err, githubapp.ErrNotInstalled)
}

func TestGitHubFailuresDoNotLeakItsText(t *testing.T) {
	key, pkcs1, _ := newKey(t)

	f := newFake(t, key)
	f.mintStatus = http.StatusInternalServerError
	_, err := newClient(t, f, pkcs1).InstallationToken(t.Context(), "o", "r")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret detail")
	assert.Contains(t, err.Error(), "500")

	f = newFake(t, key)
	f.mintStatus = http.StatusMultipleChoices
	_, err = newClient(t, f, pkcs1).InstallationToken(t.Context(), "o", "r")
	require.ErrorContains(t, err, "status 300", "a redirect is not a token")

	f = newFake(t, key)
	f.installStatus = http.StatusBadGateway
	_, err = newClient(t, f, pkcs1).InstallationToken(t.Context(), "o", "r")
	require.Error(t, err)
	assert.NotErrorIs(t, err, githubapp.ErrNotInstalled)

	for name, body := range map[string]string{
		"not json": `nope`,
		"no token": `{"token":"","expires_at":"2026-11-04T10:00:00Z"}`,
	} {
		f = newFake(t, key)
		f.mintBody = body
		_, err = newClient(t, f, pkcs1).InstallationToken(t.Context(), "o", "r")
		assert.Error(t, err, name)
	}

	// An installation answer without an id.
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/",
		func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{}`) },
	)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := githubapp.New("12345", pkcs1, githubapp.WithBaseURL(srv.URL))
	require.NoError(t, err)
	_, err = c.InstallationToken(t.Context(), "o", "r")
	require.Error(t, err)
}

func TestARepositoryNameCannotReachTheURLPath(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	c := newClient(t, f, pkcs1)
	tooLong := strings.Repeat("a", 101)
	for _, bad := range []string{"", ".", "..", "a/b", "..%2f", "a b", "x?y", tooLong} {
		_, err := c.InstallationToken(t.Context(), bad, "r")
		require.Error(t, err, bad)
		_, err = c.InstallationToken(t.Context(), "o", bad)
		require.Error(t, err, bad)
	}
	assert.Zero(t, f.installations.Load())
}

func TestNewNeedsAnIDAndAUsableKey(t *testing.T) {
	_, pkcs1, _ := newKey(t)
	_, err := githubapp.New("", pkcs1)
	require.ErrorIs(t, err, githubapp.ErrNotConfigured)
	_, err = githubapp.New("1", "")
	require.ErrorIs(t, err, githubapp.ErrNotConfigured)
	_, err = githubapp.New("1", "not pem")
	require.ErrorIs(t, err, githubapp.ErrNotConfigured)

	// A valid PKCS#8 key that is not RSA.
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(ec)
	require.NoError(t, err)
	_, err = githubapp.New("1", string(pem.EncodeToMemory(
		&pem.Block{Type: "PRIVATE KEY", Bytes: der})))
	require.ErrorIs(t, err, githubapp.ErrNotConfigured)

	// A PEM that is not a key at all.
	junk := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("garbage")})
	_, err = githubapp.New("1", string(junk))
	require.ErrorIs(t, err, githubapp.ErrNotConfigured)
}

func TestAKeyWhoseNewlinesArriveEscapedStillParses(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	f := newFake(t, key)
	escaped := strings.ReplaceAll(strings.TrimSpace(pkcs1), "\n", `\n`)
	require.NotContains(t, escaped, "\n")
	c := newClient(t, f, escaped)
	_, err := c.InstallationToken(t.Context(), "o", "r")
	require.NoError(t, err)
}

func TestAStaleInstallationIsForgottenWhateverGitHubAnswers(t *testing.T) {
	key, pkcs1, _ := newKey(t)
	for _, status := range []int{http.StatusForbidden, http.StatusUnprocessableEntity} {
		f := newFake(t, key)
		c := newClient(t, f, pkcs1)
		_, err := c.InstallationToken(t.Context(), "o", "r")
		require.NoError(t, err)

		f.mintStatus = status
		_, err = c.InstallationToken(t.Context(), "o", "r")
		require.Error(t, err)
		assert.NotErrorIs(t, err, githubapp.ErrNotInstalled, status)

		f.mintStatus = http.StatusCreated
		_, err = c.InstallationToken(t.Context(), "o", "r")
		require.NoError(t, err)
		assert.EqualValues(t, 2, f.installations.Load(), "looked up again after %d", status)
	}
}

func TestARequestThatCannotBeSentFails(t *testing.T) {
	_, pkcs1, _ := newKey(t)
	c, err := githubapp.New("1", pkcs1, githubapp.WithBaseURL("http://127.0.0.1:1"))
	require.NoError(t, err)
	_, err = c.InstallationToken(t.Context(), "o", "r")
	require.Error(t, err)
}
