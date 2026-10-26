package oauth2as_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ory/fosite"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/oauth2as"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestWriteV049Fixture regenerates v049FixturePath; it only makes sense while
// go.mod pins github.com/ory/fosite v0.49.0.
func TestWriteV049Fixture(t *testing.T) {
	if os.Getenv("OAUTH2AS_WRITE_V049_FIXTURE") == "" {
		t.Skip("set OAUTH2AS_WRITE_V049_FIXTURE=1 to regenerate")
	}
	ctx := context.Background()
	start := time.Now().Add(-time.Second)

	cfg := testhelper.NewTestConfig()
	cfg.OAuthHMACSecret = v049HMACSecret
	srv := newOAuth2asTestServerWithConfig(t, cfg)

	oidcClient := seedV049OIDCClient(t, srv)
	mcpClient := srv.registerClient(t)
	require.NoError(t, oauth2as.EnsureRoutinesClientSecret(
		ctx, srv.db, v049RoutinesSecret, nil,
	))

	var f v049Fixture
	f.MCP = srv.v049CodeGrant(t, mcpClient, "", [2]string{})

	f.Rotated = srv.v049CodeGrant(t, mcpClient, "", [2]string{})
	refreshResp, refreshed := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {f.Rotated.RefreshToken},
		"client_id":     {mcpClient.ID},
	})
	require.Equal(t, http.StatusOK, refreshResp.StatusCode)
	f.Rotated.StaleRefreshToken = f.Rotated.RefreshToken
	f.Rotated.AccessToken = refreshed.AccessToken
	f.Rotated.RefreshToken = refreshed.RefreshToken

	f.OIDC = srv.v049CodeGrant(
		t, oidcClient, v049OIDCScope, [2]string{oidcClient.ID, v049OIDCClientSecret},
	)

	verifier, challenge := pkcePair(t)
	f.PendingCode = v049Code{
		ClientID: oidcClient.ID,
		Code: srv.authorizeAndGetCodeWithScope(
			t, oidcClient, challenge, "v049-pending-state", v049OIDCScope,
		),
		Verifier:    verifier,
		RedirectURI: v049OIDCRedirectURI,
	}

	ccResp, cc := srv.exchangeOIDCToken(t, clientCredentialsForm(),
		[2]string{oauth2as.RoutinesClientID, v049RoutinesSecret})
	require.Equal(t, http.StatusOK, ccResp.StatusCode)
	f.ClientCredentials = v049Tokens{
		ClientID:          oauth2as.RoutinesClientID,
		AccessToken:       cc.AccessToken,
		RefreshToken:      "",
		StaleRefreshToken: "",
	}

	f.CapturedAt = time.Now().UTC()
	f.Rows = dumpV049Rows(t, srv, start, []string{mcpClient.ID, oidcClient.ID})

	out, err := json.MarshalIndent(f, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll("testdata", 0o750))
	require.NoError(t, os.WriteFile(v049FixturePath, append(out, '\n'), 0o600))
}

func seedV049OIDCClient(
	t *testing.T, srv *oauth2asTestServer,
) *fosite.DefaultClient {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(v049OIDCClientSecret), bcrypt.MinCost,
	)
	require.NoError(t, err)
	_, err = srv.db.Exec(context.Background(), `
		INSERT INTO auth.oauth2_clients
			(id, secret_hash, redirect_uris, grant_types, response_types, scopes,
			 public, client_name)
		VALUES ($1, $2, ARRAY[$3], ARRAY['authorization_code', 'refresh_token'],
			ARRAY['code'], ARRAY['openid', 'profile', 'email', 'offline_access'],
			false, 'fosite v0.49.0 fixture')
		ON CONFLICT (id) DO UPDATE SET secret_hash = EXCLUDED.secret_hash
	`, v049OIDCClientID, string(hash), v049OIDCRedirectURI)
	require.NoError(t, err)

	//nolint:exhaustruct //only the fields the flow helpers read
	return &fosite.DefaultClient{
		ID:           v049OIDCClientID,
		RedirectURIs: []string{v049OIDCRedirectURI},
		Scopes:       []string{"openid", "profile", "email", "offline_access"},
	}
}

// v049CodeGrant runs authorize + code exchange; basicAuth is empty for a
// public client.
func (s *oauth2asTestServer) v049CodeGrant(
	t *testing.T, client *fosite.DefaultClient, scope string, basicAuth [2]string,
) v049Tokens {
	t.Helper()
	verifier, challenge := pkcePair(t)
	code := s.authorizeAndGetCodeWithScope(
		t, client, challenge, "v049-fixture-state", scope,
	)
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"code_verifier": {verifier},
	}
	if basicAuth[0] == "" {
		form.Set("client_id", client.ID)
	}
	resp, out := s.exchangeOIDCToken(t, form, basicAuth)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, out.RefreshToken)
	return v049Tokens{
		ClientID:          client.ID,
		AccessToken:       out.AccessToken,
		RefreshToken:      out.RefreshToken,
		StaleRefreshToken: "",
	}
}

func dumpV049Rows(
	t *testing.T, srv *oauth2asTestServer, since time.Time, clientIDs []string,
) map[string][]json.RawMessage {
	t.Helper()
	allClients := append([]string{oauth2as.RoutinesClientID}, clientIDs...)
	rows := make(map[string][]json.RawMessage, len(v049Tables))
	for _, table := range v049Tables {
		query := `SELECT to_jsonb(t) FROM auth.` + table + ` t
			WHERE created_at >= $1 AND client_id = ANY($2)
			ORDER BY created_at, signature`
		args := []any{since, allClients}
		switch table {
		case "oauth2_clients":
			query = `SELECT to_jsonb(t) FROM auth.oauth2_clients t
				WHERE id = ANY($1) ORDER BY id`
			args = []any{clientIDs}
		case "oauth2_pkce_requests":
			query = `SELECT to_jsonb(t) FROM auth.oauth2_pkce_requests t
				WHERE created_at >= $1 AND request->>'client_id' = ANY($2)
				ORDER BY created_at, signature`
		}
		res, err := srv.db.Query(context.Background(), query, args...)
		require.NoError(t, err)
		for res.Next() {
			var row json.RawMessage
			require.NoError(t, res.Scan(&row))
			rows[table] = append(rows[table], row)
		}
		require.NoError(t, res.Err())
		res.Close()
	}
	return rows
}
