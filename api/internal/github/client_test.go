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
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

const (
	realBaseURL = "https://api.github.com"
	testRepo    = "xdoubleu/tools.xdoubleu.com"
)

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

func TestListFailingPullRequests_ReturnsOnlyPRsWithFailingChecks(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/" + testRepo + "/pulls":
				_, _ = w.Write([]byte(`[
					{"number":1,"title":"Fix bug","html_url":"https://gh/pr/1",
					 "updated_at":"2026-07-01T10:00:00Z",
					 "user":{"login":"alice"},"head":{"sha":"sha1"}},
					{"number":2,"title":"Add feature","html_url":"https://gh/pr/2",
					 "updated_at":"2026-07-02T10:00:00Z",
					 "user":{"login":"bob"},"head":{"sha":"sha2"}}
				]`))
			case "/repos/" + testRepo + "/commits/sha1/check-runs":
				_, _ = w.Write([]byte(`{"check_runs":[
					{"name":"ci-pass","status":"completed","conclusion":"failure",
					 "html_url":"https://gh/checks/1"}
				]}`))
			case "/repos/" + testRepo + "/commits/sha2/check-runs":
				_, _ = w.Write([]byte(`{"check_runs":[
					{"name":"ci-pass","status":"completed","conclusion":"success",
					 "html_url":"https://gh/checks/2"}
				]}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	defer cleanup()

	prs, err := newClient().ListFailingPullRequests(context.Background())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, int64(1), prs[0].Number)
	assert.Equal(t, "Fix bug", prs[0].Title)
	assert.Equal(t, "alice", prs[0].Author)
	require.Len(t, prs[0].FailingChecks, 1)
	assert.Equal(t, "ci-pass", prs[0].FailingChecks[0].Name)
	assert.Equal(t, "failure", prs[0].FailingChecks[0].Conclusion)
}

func TestListFailingPullRequests_CheckRunsUpstreamError(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/" + testRepo + "/pulls":
				_, _ = w.Write([]byte(`[
					{"number":1,"title":"Fix bug","html_url":"u",
					 "updated_at":"2026-07-01T10:00:00Z",
					 "user":{"login":"alice"},"head":{"sha":"sha1"}}
				]`))
			default:
				w.WriteHeader(http.StatusUnauthorized)
			}
		}))
	defer cleanup()

	_, err := newClient().ListFailingPullRequests(context.Background())
	require.Error(t, err)
}

func TestListFailingPullRequests_TokenLookupError(t *testing.T) {
	c := github.New(
		logging.NewNopLogger(),
		func(context.Context) (string, error) { return "", assert.AnError },
		configWithRepo(testRepo),
	)
	_, err := c.ListFailingPullRequests(context.Background())
	require.ErrorIs(t, err, assert.AnError)
}

func TestListFailingPullRequests_SkipsInProgressChecks(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/" + testRepo + "/pulls":
				_, _ = w.Write([]byte(`[
					{"number":1,"title":"WIP","html_url":"u","updated_at":"2026-07-01T10:00:00Z",
					 "user":{"login":"alice"},"head":{"sha":"sha1"}}
				]`))
			case "/repos/" + testRepo + "/commits/sha1/check-runs":
				_, _ = w.Write([]byte(`{"check_runs":[
					{"name":"ci","status":"in_progress","conclusion":"",
					 "html_url":"u"}
				]}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	defer cleanup()

	prs, err := newClient().ListFailingPullRequests(context.Background())
	require.NoError(t, err)
	assert.Empty(t, prs)
}

func TestListFailingPullRequests_NotConfigured_NoConnection(t *testing.T) {
	c := github.New(logging.NewNopLogger(), stubToken("token"), configNotConnected())
	_, err := c.ListFailingPullRequests(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestListFailingPullRequests_NotConfigured_NotConnected(t *testing.T) {
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
	_, err := c.ListFailingPullRequests(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
	assert.False(t, called, "must not hit the API when unconfigured")
}

func TestListFailingPullRequests_CachesResult(t *testing.T) {
	requests := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/repos/"+testRepo+"/pulls" {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
	defer cleanup()

	c := newClient()
	_, err := c.ListFailingPullRequests(context.Background())
	require.NoError(t, err)
	_, err = c.ListFailingPullRequests(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, requests, "second call must be served from cache")
}

func TestListFailingPullRequests_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
	defer cleanup()

	_, err := newClient().ListFailingPullRequests(context.Background())
	require.Error(t, err)
	assert.Equal(t, 4, attempts, "5xx must retry up to maxAttempts")
}

func jsonHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}
