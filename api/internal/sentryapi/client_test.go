package sentryapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/sentryapi"
)

const realBaseURL = "https://sentry.io"

func stubToken(token string) oauthconn.TokenFunc {
	return func(context.Context) (string, error) { return token, nil }
}

func stubNotConnected() oauthconn.TokenFunc {
	return func(context.Context) (string, error) { return "", oauthconn.ErrNotConnected }
}

func TestMain(m *testing.M) {
	sentryapi.SetBackoffBase(1 * time.Millisecond)
	os.Exit(m.Run())
}

func buildServer(handler http.Handler) func() {
	srv := httptest.NewServer(handler)
	sentryapi.SetBaseURL(srv.URL)
	return func() {
		srv.Close()
		sentryapi.SetBaseURL(realBaseURL)
	}
}

func newClient() sentryapi.Client {
	return sentryapi.New(logging.NewNopLogger(), "org", "proj", stubToken("token"))
}

func TestListUnresolvedIssues_ParsesPayload(t *testing.T) {
	body := `[
		{"id":"42","title":"boom","culprit":"main.go","permalink":"https://s/42",
		 "count":"17","lastSeen":"2026-07-10T09:00:00Z","level":"error"}
	]`

	var gotPath, gotQuery, authHeader string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotQuery = r.URL.Query().Get("query")
			authHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
	defer cleanup()

	issues, err := newClient().ListUnresolvedIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "42", issues[0].ID)
	assert.Equal(t, "boom", issues[0].Title)
	assert.Equal(t, int64(17), issues[0].Count)
	assert.Equal(t, "error", issues[0].Level)
	assert.True(t, strings.HasSuffix(gotPath, "/api/0/projects/org/proj/issues/"))
	assert.Equal(t, "is:unresolved", gotQuery)
	assert.Equal(t, "Bearer token", authHeader)
}

func TestListUnresolvedIssues_NotConfigured(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	cases := []sentryapi.Client{
		sentryapi.New(logging.NewNopLogger(), "", "proj", stubToken("token")),
		sentryapi.New(logging.NewNopLogger(), "org", "", stubToken("token")),
		sentryapi.New(logging.NewNopLogger(), "org", "proj", stubNotConnected()),
	}
	for _, c := range cases {
		_, err := c.ListUnresolvedIssues(context.Background())
		require.ErrorIs(t, err, sentryapi.ErrNotConfigured)
	}
	assert.False(t, called, "must not hit the API when unconfigured")
}

func TestListUnresolvedIssues_CachesResult(t *testing.T) {
	requests := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"1","title":"x","count":"1"}]`))
		}))
	defer cleanup()

	c := newClient()
	_, err := c.ListUnresolvedIssues(context.Background())
	require.NoError(t, err)
	_, err = c.ListUnresolvedIssues(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, requests, "second call must be served from cache")
}

func TestListUnresolvedIssues_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusInternalServerError)
		}))
	defer cleanup()

	_, err := newClient().ListUnresolvedIssues(context.Background())
	require.Error(t, err)
	assert.Equal(t, 4, attempts)
}

func TestListUnresolvedIssues_NonRetryable4xx(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusForbidden)
		}))
	defer cleanup()

	_, err := newClient().ListUnresolvedIssues(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, attempts)
}

func TestListUnresolvedIssues_NetworkError(t *testing.T) {
	sentryapi.SetBaseURL("http://127.0.0.1:1")
	defer sentryapi.SetBaseURL(realBaseURL)

	_, err := newClient().ListUnresolvedIssues(context.Background())
	require.Error(t, err)
}
