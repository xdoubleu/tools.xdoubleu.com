package oauth2as_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ory/fosite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/oauth2as"
)

const grafanaTestSecret = "grafana-test-client-secret-0123456789"

// grafanaConfidentialClient reconciles the seeded static Grafana client's
// secret to a known value and returns a client struct for driving the flow.
func grafanaConfidentialClient(
	t *testing.T,
	srv *oauth2asTestServer,
) *fosite.DefaultClient {
	t.Helper()
	require.NoError(t, oauth2as.EnsureGrafanaClientSecret(
		context.Background(), srv.db, grafanaTestSecret, nil,
	))
	//nolint:exhaustruct //only the fields the flow helpers read
	return &fosite.DefaultClient{
		ID: oauth2as.GrafanaClientID,
		RedirectURIs: []string{
			"https://tools.xdoubleu.com/grafana/login/generic_oauth",
		},
		GrantTypes:    []string{"authorization_code", "refresh_token"},
		ResponseTypes: []string{"code"},
		Scopes:        oauth2as.SupportedScopes,
		Public:        false,
	}
}

type oidcTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
}

func (s *oauth2asTestServer) exchangeOIDCToken(
	t *testing.T, form url.Values, basicAuth [2]string,
) (*http.Response, oidcTokenResponse) {
	t.Helper()
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		s.ts.URL+"/oauth2/token", strings.NewReader(form.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicAuth[0] != "" {
		req.SetBasicAuth(basicAuth[0], basicAuth[1])
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var out oidcTokenResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestOIDC_AuthorizationCodeFlow_IssuesVerifiableIDToken(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := grafanaConfidentialClient(t, srv)

	verifier, challenge := pkcePair(t)
	code := srv.authorizeAndGetCodeWithScope(
		t,
		client,
		challenge,
		"oidc-state-abcdef01",
		"openid email profile offline_access",
	)

	resp, out := srv.exchangeOIDCToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"client_secret": {grafanaTestSecret},
		"code_verifier": {verifier},
	}, [2]string{})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, out.AccessToken)
	require.NotEmpty(t, out.RefreshToken, "offline_access should yield a refresh token")
	require.NotEmpty(t, out.IDToken, "openid scope should yield an id token")

	// The ID token verifies against the server's signing key with the kid
	// advertised by /oauth2/jwks.
	tok, err := jwt.Parse(out.IDToken, func(tok *jwt.Token) (any, error) {
		assert.Equal(t, "RS256", tok.Method.Alg())
		assert.Equal(t, oauth2as.OIDCKeyID(srv.key), tok.Header["kid"])
		return &srv.key.PublicKey, nil
	})
	require.NoError(t, err)
	require.True(t, tok.Valid)

	claims, ok := tok.Claims.(jwt.MapClaims)
	require.True(t, ok)
	assert.Equal(t, srv.userID, claims["sub"])
	assert.Equal(t, "e2e-user@example.com", claims["email"])
	assert.Equal(t, true, claims["email_verified"])
	assert.Equal(t, "Admin", claims["role"], "admin user maps to the Admin role claim")
	assert.Equal(t, "e2e-user@example.com", claims["preferred_username"])
	assert.Contains(t, claims["aud"], client.ID)

	// The JWKS endpoint exposes exactly the one public key.
	jwksResp, err := http.Get(srv.ts.URL + "/oauth2/jwks")
	require.NoError(t, err)
	defer jwksResp.Body.Close()
	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	require.NoError(t, json.NewDecoder(jwksResp.Body).Decode(&jwks))
	require.Len(t, jwks.Keys, 1)
	assert.Equal(t, "RS256", jwks.Keys[0]["alg"])
	assert.Equal(t, oauth2as.OIDCKeyID(srv.key), jwks.Keys[0]["kid"])
}

func TestOIDC_ConfidentialClient_SecretIsEnforced(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := grafanaConfidentialClient(t, srv)

	exchange := func(t *testing.T, secretForm string, basic [2]string) int {
		t.Helper()
		verifier, challenge := pkcePair(t)
		code := srv.authorizeAndGetCodeWithScope(
			t, client, challenge, "cc-state-abcdef012", "openid email",
		)
		form := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"redirect_uri":  {client.RedirectURIs[0]},
			"client_id":     {client.ID},
			"code_verifier": {verifier},
		}
		if secretForm != "" {
			form.Set("client_secret", secretForm)
		}
		resp, _ := srv.exchangeOIDCToken(t, form, basic)
		return resp.StatusCode
	}

	t.Run("client_secret_post succeeds", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, exchange(t, grafanaTestSecret, [2]string{}))
	})
	t.Run("client_secret_basic succeeds", func(t *testing.T) {
		assert.Equal(t, http.StatusOK,
			exchange(t, "", [2]string{client.ID, grafanaTestSecret}))
	})
	t.Run("wrong secret is rejected", func(t *testing.T) {
		assert.Equal(
			t,
			http.StatusUnauthorized,
			exchange(t, "wrong-secret", [2]string{}),
		)
	})
	t.Run("missing secret is rejected", func(t *testing.T) {
		assert.NotEqual(t, http.StatusOK, exchange(t, "", [2]string{}))
	})
}

func TestEnsureGrafanaClientSecret_LoggedBranches(t *testing.T) {
	_, db := newTestStore(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Empty secret: logs, does not write.
	require.NoError(t, oauth2as.EnsureGrafanaClientSecret(ctx, db, "", logger))

	// Missing client row (migration not yet applied): logged, not returned.
	_, err := db.Exec(ctx,
		`DELETE FROM auth.oauth2_clients WHERE id = $1`, oauth2as.GrafanaClientID,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(),
			`INSERT INTO auth.oauth2_clients
				(id, redirect_uris, grant_types, response_types, scopes, public, client_name)
			 VALUES ('grafana', ARRAY['https://tools.xdoubleu.com/grafana/login/generic_oauth'],
				ARRAY['authorization_code','refresh_token'], ARRAY['code'],
				ARRAY['openid','profile','email','offline_access'], FALSE, 'Grafana')
			 ON CONFLICT (id) DO NOTHING`)
	})
	require.NoError(t,
		oauth2as.EnsureGrafanaClientSecret(ctx, db, "some-secret", logger))

	// Row present again, secret set: logs the reconcile.
	_, err = db.Exec(ctx,
		`INSERT INTO auth.oauth2_clients
			(id, redirect_uris, grant_types, response_types, scopes, public, client_name)
		 VALUES ('grafana', ARRAY['https://tools.xdoubleu.com/grafana/login/generic_oauth'],
			ARRAY['authorization_code','refresh_token'], ARRAY['code'],
			ARRAY['openid','profile','email','offline_access'], FALSE, 'Grafana')
		 ON CONFLICT (id) DO NOTHING`)
	require.NoError(t, err)
	require.NoError(t,
		oauth2as.EnsureGrafanaClientSecret(ctx, db, "reconciled-secret", logger))
}

func TestEnsureGrafanaClientSecret_Idempotent(t *testing.T) {
	_, db := newTestStore(t)
	ctx := context.Background()

	// Reset to the migration's starting state (NULL secret_hash).
	_, err := db.Exec(ctx,
		`UPDATE auth.oauth2_clients SET secret_hash = NULL WHERE id = $1`,
		oauth2as.GrafanaClientID,
	)
	require.NoError(t, err)

	require.NoError(t, oauth2as.EnsureGrafanaClientSecret(ctx, db, "s3cret-value", nil))
	var first string
	require.NoError(t, db.QueryRow(ctx,
		`SELECT secret_hash FROM auth.oauth2_clients WHERE id = $1`,
		oauth2as.GrafanaClientID,
	).Scan(&first))
	assert.True(t, strings.HasPrefix(first, "$2"), "stored value is a bcrypt hash")

	// Second run with the same secret leaves the hash untouched.
	require.NoError(t, oauth2as.EnsureGrafanaClientSecret(ctx, db, "s3cret-value", nil))
	var second string
	require.NoError(t, db.QueryRow(ctx,
		`SELECT secret_hash FROM auth.oauth2_clients WHERE id = $1`,
		oauth2as.GrafanaClientID,
	).Scan(&second))
	assert.Equal(t, first, second)

	// A rotated secret is written through.
	require.NoError(
		t,
		oauth2as.EnsureGrafanaClientSecret(ctx, db, "rotated-secret", nil),
	)
	var third string
	require.NoError(t, db.QueryRow(ctx,
		`SELECT secret_hash FROM auth.oauth2_clients WHERE id = $1`,
		oauth2as.GrafanaClientID,
	).Scan(&third))
	assert.NotEqual(t, first, third)

	// An empty secret is a no-op, not a wipe.
	require.NoError(t, oauth2as.EnsureGrafanaClientSecret(ctx, db, "", nil))
	var fourth string
	require.NoError(t, db.QueryRow(ctx,
		`SELECT secret_hash FROM auth.oauth2_clients WHERE id = $1`,
		oauth2as.GrafanaClientID,
	).Scan(&fourth))
	assert.Equal(t, third, fourth)
}
