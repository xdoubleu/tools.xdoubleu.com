package oauth2as_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/oauth2as"
)

const routinesTestSecret = "routines-test-client-secret-0123456789"

func clientCredentialsForm() url.Values {
	return url.Values{"grant_type": {"client_credentials"}}
}

func TestClientCredentials_RoutinesClient_ActsAsServiceUser(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	ctx := context.Background()
	require.NoError(t, oauth2as.EnsureRoutinesClientSecret(
		ctx, srv.db, routinesTestSecret, nil,
	))

	resp, tok := srv.exchangeOIDCToken(t, clientCredentialsForm(),
		[2]string{oauth2as.RoutinesClientID, routinesTestSecret})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, tok.RefreshToken)

	subject, err := oauth2as.NewTokenResolver(srv.provider).
		ResolveAccessToken(ctx, tok.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, oauth2as.RoutinesServiceUserID, subject)

	t.Run("wrong secret is rejected", func(t *testing.T) {
		wrong, _ := srv.exchangeOIDCToken(t, clientCredentialsForm(),
			[2]string{oauth2as.RoutinesClientID, "wrong-secret"})
		assert.Equal(t, http.StatusUnauthorized, wrong.StatusCode)
	})
}

// TestClientCredentials_OtherClientsRefused: only a client registered with the
// grant and mapped to a machine identity gets a token.
func TestClientCredentials_OtherClientsRefused(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	ctx := context.Background()

	t.Run("grafana lacks the grant", func(t *testing.T) {
		client := grafanaConfidentialClient(t, srv)
		resp, _ := srv.exchangeOIDCToken(t, clientCredentialsForm(),
			[2]string{client.ID, grafanaTestSecret})
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("public client", func(t *testing.T) {
		client := srv.registerClient(t)
		form := clientCredentialsForm()
		form.Set("client_id", client.ID)
		resp, _ := srv.exchangeOIDCToken(t, form, [2]string{})
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("confidential client without a machine identity", func(t *testing.T) {
		const id = "unmapped-machine-client"
		hash, err := bcrypt.GenerateFromPassword(
			[]byte(routinesTestSecret), bcrypt.MinCost,
		)
		require.NoError(t, err)
		_, err = srv.db.Exec(ctx, `
			INSERT INTO auth.oauth2_clients (id, secret_hash, grant_types, public)
			VALUES ($1, $2, ARRAY['client_credentials'], FALSE)
			ON CONFLICT (id) DO NOTHING`, id, string(hash))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = srv.db.Exec(context.Background(),
				`DELETE FROM auth.oauth2_clients WHERE id = $1`, id)
		})

		resp, _ := srv.exchangeOIDCToken(t, clientCredentialsForm(),
			[2]string{id, routinesTestSecret})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
