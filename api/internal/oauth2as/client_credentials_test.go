package oauth2as_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

// tokenServer serves /oauth2/token with the given machine-subject lookup.
func tokenServer(
	t *testing.T, srv *oauth2asTestServer, lookup oauth2as.MachineSubjects,
) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", oauth2as.TokenHandler(
		srv.provider, oauth2as.NewMachineSubjects(lookup),
		slog.New(slog.DiscardHandler),
	))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func postClientCredentials(
	t *testing.T, ts *httptest.Server, clientID, secret string,
) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, ts.URL+"/oauth2/token",
		strings.NewReader(clientCredentialsForm().Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, secret)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body.AccessToken
}

// insertMachineClient adds a confidential client_credentials client.
func insertMachineClient(t *testing.T, srv *oauth2asTestServer, id string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(routinesTestSecret), bcrypt.MinCost)
	require.NoError(t, err)
	_, err = srv.db.Exec(context.Background(), `
		INSERT INTO auth.oauth2_clients (id, secret_hash, grant_types, public)
		VALUES ($1, $2, ARRAY['client_credentials'], FALSE)`, id, string(hash))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = srv.db.Exec(context.Background(),
			`DELETE FROM auth.oauth2_clients WHERE id = $1`, id)
	})
}

func TestClientCredentials_LookedUpClientActsAsItsSubject(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	const id = "looked-up-machine-client"
	insertMachineClient(t, srv, id)
	const subjectID = "5a1c0a1e-2d0e-4a57-9b56-6f8d1d3b9a10"
	lookup := func(_ context.Context, clientID string) (string, bool, error) {
		return subjectID, clientID == id, nil
	}
	ts := tokenServer(t, srv, lookup)

	status, token := postClientCredentials(t, ts, id, routinesTestSecret)
	require.Equal(t, http.StatusOK, status)
	subject, err := oauth2as.NewTokenResolver(srv.provider).
		ResolveAccessToken(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, subjectID, subject)
}

func TestClientCredentials_LookupErrorIsAServerError(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	const id = "failing-lookup-machine-client"
	insertMachineClient(t, srv, id)
	ts := tokenServer(t, srv, func(context.Context, string) (string, bool, error) {
		return "", false, errors.New("database unavailable")
	})

	status, token := postClientCredentials(t, ts, id, routinesTestSecret)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Empty(t, token)
}

func TestMachineSubjects(t *testing.T) {
	ctx := context.Background()
	calls := 0
	lookup := func(_ context.Context, clientID string) (string, bool, error) {
		calls++
		return "agent-user", clientID == "agent-1", nil
	}

	subjects := oauth2as.NewMachineSubjects(lookup)
	subject, ok, err := subjects(ctx, oauth2as.RoutinesClientID)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, oauth2as.RoutinesServiceUserID, subject)
	assert.Zero(t, calls, "the routines client never reaches the lookup")

	subject, ok, err = subjects(ctx, "agent-1")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "agent-user", subject)

	_, ok, err = subjects(ctx, "someone-else")
	require.NoError(t, err)
	assert.False(t, ok || calls != 2)

	_, ok, err = oauth2as.NewMachineSubjects(nil)(ctx, "agent-1")
	require.NoError(t, err)
	assert.False(t, ok, "without a lookup only routines has an identity")
}
