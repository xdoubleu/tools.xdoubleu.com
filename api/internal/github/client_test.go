package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

const (
	realBaseURL = "https://api.github.com"
	testRepo    = "xdoubleu/tools.xdoubleu.com"
)

//nolint:unparam // always "token" in practice; kept as a param for clarity
func stubToken(token string) oauthconn.TokenFunc {
	return func(context.Context) (string, error) { return token, nil }
}

func stubNotConnected() oauthconn.TokenFunc {
	return func(context.Context) (string, error) { return "", oauthconn.ErrNotConnected }
}

// stubConfigStore stands in for *repositories.OAuthConnectionsRepository.
type stubConfigStore struct {
	conn *models.OAuthConnection
	err  error
}

func (s stubConfigStore) Get(
	context.Context, models.OAuthProvider,
) (*oauth2.Token, *models.OAuthConnection, error) {
	return nil, s.conn, s.err
}

func configWithRepo(repo string) stubConfigStore {
	return stubConfigStore{
		conn: &models.OAuthConnection{ //nolint:exhaustruct // test fixture
			Config: json.RawMessage(fmt.Sprintf(`{"repo":%q}`, repo)),
		},
		err: nil,
	}
}

func configNotConnected() stubConfigStore {
	//nolint:exhaustruct // conn intentionally nil: simulates "not connected"
	return stubConfigStore{err: database.ErrResourceNotFound}
}

func configWithMalformedJSON() stubConfigStore {
	return stubConfigStore{
		conn: &models.OAuthConnection{ //nolint:exhaustruct // test fixture
			Config: json.RawMessage(`not json`),
		},
		err: nil,
	}
}

func configGetError() stubConfigStore {
	//nolint:exhaustruct // conn intentionally nil: a generic DB error
	return stubConfigStore{err: assert.AnError}
}

func TestMain(m *testing.M) {
	github.SetBackoffBase(1 * time.Millisecond)
	os.Exit(m.Run())
}

// buildServer starts an httptest.Server serving handler and points the
// package-level baseURL at it. The returned func restores the real URL.
func buildServer(handler http.Handler) func() {
	srv := httptest.NewServer(handler)
	github.SetBaseURL(srv.URL)
	return func() {
		srv.Close()
		github.SetBaseURL(realBaseURL)
	}
}

func newClient() github.Client {
	return github.New(
		logging.NewNopLogger(),
		stubToken("token"),
		configWithRepo(testRepo),
	)
}

func TestListOpenIssues_ReturnsIssuesAndSkipsPRs(t *testing.T) {
	body := `[
		{"number":1,"title":"Bug","html_url":"https://gh/1","state":"open",
		 "created_at":"2026-07-01T10:00:00Z","labels":[{"name":"bug"}]},
		{"number":2,"title":"PR","html_url":"https://gh/2","state":"open",
		 "created_at":"2026-07-02T10:00:00Z","pull_request":{"url":"x"}}
	]`

	cleanup := buildServer(jsonHandler(http.StatusOK, body))
	defer cleanup()

	issues, err := newClient().ListOpenIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, int64(1), issues[0].Number)
	assert.Equal(t, "Bug", issues[0].Title)
	assert.Equal(t, "https://gh/1", issues[0].URL)
	assert.Equal(t, []string{"bug"}, issues[0].Labels)
}

func TestListOpenIssues_SendsBearerToken(t *testing.T) {
	var authHeader string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}))
	defer cleanup()

	_, err := newClient().ListOpenIssues(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer token", authHeader)
}

func TestListOpenIssues_NotConfigured_NotConnected(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	c := github.New(
		logging.NewNopLogger(),
		stubNotConnected(),
		configWithRepo(testRepo),
	)
	_, err := c.ListOpenIssues(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
	assert.False(t, called, "must not hit the API when unconfigured")
}

func TestListOpenIssues_NotConfigured_NoConnection(t *testing.T) {
	c := github.New(logging.NewNopLogger(), stubToken("token"), configNotConnected())
	_, err := c.ListOpenIssues(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestListOpenIssues_NotConfigured_NoRepoPicked(t *testing.T) {
	c := github.New(logging.NewNopLogger(), stubToken("token"), configWithRepo(""))
	_, err := c.ListOpenIssues(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestListOpenIssues_MalformedConfig(t *testing.T) {
	c := github.New(
		logging.NewNopLogger(),
		stubToken("token"),
		configWithMalformedJSON(),
	)
	_, err := c.ListOpenIssues(context.Background())
	require.Error(t, err)
	require.NotErrorIs(t, err, github.ErrNotConfigured)
}

func TestListOpenIssues_ConfigLookupError(t *testing.T) {
	c := github.New(logging.NewNopLogger(), stubToken("token"), configGetError())
	_, err := c.ListOpenIssues(context.Background())
	require.ErrorIs(t, err, assert.AnError)
}

func TestListOpenIssues_CachesResult(t *testing.T) {
	requests := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`[{"number":1,"title":"A","html_url":"u","state":"open"}]`,
			))
		}))
	defer cleanup()

	c := newClient()
	_, err := c.ListOpenIssues(context.Background())
	require.NoError(t, err)
	_, err = c.ListOpenIssues(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, requests, "second call must be served from cache")
}

func TestListOpenIssues_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
	defer cleanup()

	_, err := newClient().ListOpenIssues(context.Background())
	require.Error(t, err)
	assert.Equal(t, 4, attempts, "5xx must retry up to maxAttempts")
}

func TestListOpenIssues_NonRetryable4xx(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusUnauthorized)
		}))
	defer cleanup()

	_, err := newClient().ListOpenIssues(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, attempts, "4xx must not retry")
}

func TestListOpenIssues_ContextCanceled(t *testing.T) {
	cleanup := buildServer(jsonHandler(http.StatusOK, `[]`))
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newClient().ListOpenIssues(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestListOpenIssues_NetworkError(t *testing.T) {
	github.SetBaseURL("http://127.0.0.1:1")
	defer github.SetBaseURL(realBaseURL)

	_, err := newClient().ListOpenIssues(context.Background())
	require.Error(t, err)
}

func jsonHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}
