package sentryapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/sentryapi"
)

const realBaseURL = "https://sentry.io"

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

func configWith(org string, projects ...string) stubConfigStore {
	raw, _ := json.Marshal(struct {
		Org      string   `json:"org"`
		Projects []string `json:"projects"`
	}{Org: org, Projects: projects})
	return stubConfigStore{
		conn: &models.OAuthConnection{Config: raw}, //nolint:exhaustruct // test fixture
		err:  nil,
	}
}

func configNotConnected() stubConfigStore {
	//nolint:exhaustruct // conn intentionally nil: simulates "not connected"
	return stubConfigStore{err: database.ErrResourceNotFound}
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
	return sentryapi.New(
		logging.NewNopLogger(), stubToken("token"), configWith("org", "proj"),
	)
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
	assert.Equal(t, "proj", issues[0].Project)
	assert.True(t, strings.HasSuffix(gotPath, "/api/0/projects/org/proj/issues/"))
	assert.Equal(t, "is:unresolved", gotQuery)
	assert.Equal(t, "Bearer token", authHeader)
}

func TestListUnresolvedIssues_MergesMultipleProjectsSortedByLastSeen(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/proj-a/") {
				_, _ = w.Write([]byte(
					`[{"id":"a1","title":"old","count":"1","lastSeen":"2026-07-01T00:00:00Z"}]`,
				))
				return
			}
			_, _ = w.Write([]byte(
				`[{"id":"b1","title":"new","count":"1","lastSeen":"2026-07-10T00:00:00Z"}]`,
			))
		}))
	defer cleanup()

	c := sentryapi.New(
		logging.NewNopLogger(),
		stubToken("token"),
		configWith("org", "proj-a", "proj-b"),
	)
	issues, err := c.ListUnresolvedIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 2)
	assert.Equal(t, "b1", issues[0].ID, "newest issue must sort first")
	assert.Equal(t, "proj-b", issues[0].Project)
	assert.Equal(t, "a1", issues[1].ID)
	assert.Equal(t, "proj-a", issues[1].Project)
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
		sentryapi.New(
			logging.NewNopLogger(),
			stubToken("token"),
			configWith("", "proj"),
		),
		sentryapi.New(logging.NewNopLogger(), stubToken("token"), configWith("org")),
		sentryapi.New(
			logging.NewNopLogger(),
			stubNotConnected(),
			configWith("org", "proj"),
		),
		sentryapi.New(logging.NewNopLogger(), stubToken("token"), configNotConnected()),
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
