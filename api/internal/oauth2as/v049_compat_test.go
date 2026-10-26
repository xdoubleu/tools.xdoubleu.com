package oauth2as_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	fosite "authelia.com/provider/oauth2"
	"authelia.com/provider/oauth2/handler/openid"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/oauth2as"
	"tools.xdoubleu.com/internal/testhelper"
)

func newV049TestServer(t *testing.T) *oauth2asTestServer {
	t.Helper()
	cfg := testhelper.NewTestConfig()
	cfg.OAuthHMACSecret = v049HMACSecret
	return newOAuth2asTestServerWithConfig(t, cfg)
}

func TestV049Backfill_SessionsDecodeAfterward(t *testing.T) {
	srv := newV049TestServer(t)
	ctx := context.Background()
	f := loadV049Fixture(t, srv.db)
	oidcSig := hmacSignature(t, f.OIDC.RefreshToken)
	mcpSig := hmacSignature(t, f.MCP.RefreshToken)

	_, err := srv.store.GetRefreshTokenSession(ctx, oidcSig, openid.NewDefaultSession())
	require.Error(t, err, "v0.49's RFC 3339 claim times don't decode unmigrated")

	applyBackfill(t, srv.db)
	applyBackfill(t, srv.db) // idempotent

	for _, sig := range []string{oidcSig, mcpSig} {
		session := openid.NewDefaultSession()
		_, err = srv.store.GetRefreshTokenSession(ctx, sig, session)
		require.NoError(t, err)
		assert.NotEmpty(t, session.Subject)
		require.NotNil(t, session.Claims.IssuedAt)
		assert.WithinDuration(t, time.Now(), session.Claims.IssuedAt.Time, time.Hour)
		assert.Nil(t, session.Claims.ExpirationTime, "zero exp is dropped")
	}

	claims := storedClaims(t, srv.db, "oauth2_refresh_tokens", oidcSig)
	assert.IsType(t, float64(0), claims["iat"])
	assert.IsType(t, float64(0), claims["rat"])
	assert.NotContains(t, claims, "exp")
	assert.NotContains(t, claims, "aud", "null claims are dropped")
	ext, ok := claims["ext"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Admin", ext["role"])
	assert.NotContains(t, storedClaims(t, srv.db, "oauth2_refresh_tokens", mcpSig),
		"ext", `"ext": null is dropped`)

	t.Run("down restores RFC 3339 times", func(t *testing.T) {
		tx, txErr := srv.db.Begin(ctx)
		require.NoError(t, txErr)
		defer func() { _ = tx.Rollback(ctx) }()
		_, txErr = tx.Exec(ctx, backfillSQL(t, false))
		require.NoError(t, txErr)

		var iat string
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT request #>> '{session,id_token_claims,iat}'
			FROM auth.oauth2_refresh_tokens WHERE signature = $1
		`, oidcSig).Scan(&iat))
		_, txErr = time.Parse(time.RFC3339, iat)
		require.NoError(t, txErr)
	})
}

func TestV049Tokens_StillWork(t *testing.T) {
	srv := newV049TestServer(t)
	ctx := context.Background()
	refresh := func(t *testing.T, tokens v049Tokens, token string, auth [2]string) (
		*http.Response, oidcTokenResponse,
	) {
		t.Helper()
		form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {token}}
		if auth[0] == "" {
			form.Set("client_id", tokens.ClientID)
		}
		return srv.exchangeOIDCToken(t, form, auth)
	}
	oidcAuth := [2]string{v049OIDCClientID, v049OIDCClientSecret}

	t.Run("access tokens introspect as active", func(t *testing.T) {
		f := loadV049Fixture(t, srv.db)
		applyBackfill(t, srv.db)
		resolver := oauth2as.NewTokenResolver(srv.provider)
		for _, tok := range []v049Tokens{f.MCP, f.Rotated, f.OIDC, f.ClientCredentials} {
			subject, err := resolver.ResolveAccessToken(ctx, tok.AccessToken)
			require.NoError(t, err, tok.ClientID)
			assert.NotEmpty(t, subject)
		}
		subject, err := resolver.ResolveAccessToken(ctx, f.ClientCredentials.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, oauth2as.RoutinesServiceUserID, subject)
	})

	t.Run("MCP refresh token rotates", func(t *testing.T) {
		f := loadV049Fixture(t, srv.db)
		applyBackfill(t, srv.db)
		resp, out := refresh(t, f.MCP, f.MCP.RefreshToken, [2]string{})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEqual(t, f.MCP.RefreshToken, out.RefreshToken)
		_, err := oauth2as.NewTokenResolver(srv.provider).
			ResolveAccessToken(ctx, out.AccessToken)
		require.NoError(t, err)
	})

	t.Run("OIDC refresh token rotates and keeps its claims", func(t *testing.T) {
		f := loadV049Fixture(t, srv.db)
		applyBackfill(t, srv.db)
		resp, out := refresh(t, f.OIDC, f.OIDC.RefreshToken, oidcAuth)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		claims := verifyIDToken(t, srv, out.IDToken)
		assert.Equal(t, "Admin", claims["role"])
		assert.Equal(t, "e2e-user@example.com", claims["email"])
		assert.Contains(t, claims["aud"], v049OIDCClientID)
	})

	t.Run("pending code with PKCE exchanges", func(t *testing.T) {
		f := loadV049Fixture(t, srv.db)
		applyBackfill(t, srv.db)
		resp, out := srv.exchangeOIDCToken(t, url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {f.PendingCode.Code},
			"redirect_uri":  {f.PendingCode.RedirectURI},
			"code_verifier": {f.PendingCode.Verifier},
		}, oidcAuth)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotEmpty(t, out.RefreshToken)
		assert.Equal(t, "Admin", verifyIDToken(t, srv, out.IDToken)["role"])
	})

	t.Run("reuse past the grace period revokes the chain", func(t *testing.T) {
		f := loadV049Fixture(t, srv.db)
		applyBackfill(t, srv.db)
		_, err := srv.db.Exec(ctx, `
			UPDATE auth.oauth2_refresh_tokens
			SET rotated_at = now() - interval '1 minute' WHERE signature = $1
		`, hmacSignature(t, f.Rotated.StaleRefreshToken))
		require.NoError(t, err)
		srv.logs.reset()

		resp, _ := refresh(t, f.Rotated, f.Rotated.StaleRefreshToken, [2]string{})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assertLoggedReuse(t, srv)

		resp, _ = refresh(t, f.Rotated, f.Rotated.RefreshToken, [2]string{})
		assert.NotEqual(t, http.StatusOK, resp.StatusCode, "successor is revoked")
		_, _, err = srv.provider.IntrospectToken(
			ctx, f.Rotated.AccessToken, fosite.AccessToken,
			&fosite.DefaultSession{}, //nolint:exhaustruct //filled by introspection
		)
		require.Error(t, err, "chain's access token is revoked")
	})
}

// verifyIDToken checks the signature against the server key and returns the
// claims. The kid header is the one stored with the session, which in
// production is OIDCKeyID of the same persistent key.
func verifyIDToken(t *testing.T, srv *oauth2asTestServer, raw string) jwt.MapClaims {
	t.Helper()
	require.NotEmpty(t, raw)
	tok, err := jwt.Parse(raw, func(tok *jwt.Token) (any, error) {
		assert.Equal(t, "RS256", tok.Method.Alg())
		assert.NotEmpty(t, tok.Header["kid"])
		return &srv.key.PublicKey, nil
	})
	require.NoError(t, err)
	claims, ok := tok.Claims.(jwt.MapClaims)
	require.True(t, ok)
	return claims
}

func assertLoggedReuse(t *testing.T, srv *oauth2asTestServer) {
	t.Helper()
	for _, record := range srv.logs.all() {
		if record.Message == "oauth2 request rejected" {
			assert.Equal(t, "ERROR", record.Level.String())
			return
		}
	}
	t.Fatal("reuse was not logged")
}
