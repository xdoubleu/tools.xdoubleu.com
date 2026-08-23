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

func TestListFailingPullRequests_CapturesLabelsAndHeadSHA(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/" + testRepo + "/pulls":
				_, _ = w.Write([]byte(`[
					{"number":1,"title":"Bump foo","html_url":"https://gh/pr/1",
					 "updated_at":"2026-07-01T10:00:00Z",
					 "user":{"login":"renovate[bot]"},"head":{"sha":"sha1"},
					 "labels":[{"name":"dependencies"},{"name":"go"}]}
				]`))
			case "/repos/" + testRepo + "/commits/sha1/check-runs":
				_, _ = w.Write([]byte(`{"check_runs":[
					{"name":"ci-pass","status":"completed","conclusion":"failure",
					 "html_url":"https://gh/checks/1"}
				]}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	defer cleanup()

	prs, err := newClient().ListFailingPullRequests(context.Background())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, "sha1", prs[0].HeadSHA)
	assert.Equal(t, []string{"dependencies", "go"}, prs[0].Labels)
	assert.True(t, prs[0].HasLabel("dependencies"))
	assert.False(t, prs[0].HasLabel("bug"))
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

func TestIsTransientAPIError_Timeout(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) {
			time.Sleep(50 * time.Millisecond)
		}))
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := newClient().ListFailingPullRequests(ctx)
	require.Error(t, err)
	assert.True(t, github.IsTransientAPIError(err))
}

func TestIsTransientAPIError_ServerError(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
	defer cleanup()

	_, err := newClient().ListFailingPullRequests(context.Background())
	require.Error(t, err)
	assert.False(t, github.IsTransientAPIError(err))
}

func TestListSecurityAlerts_ReturnsAlerts(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/" + testRepo + "/dependabot/alerts":
				_, _ = w.Write([]byte(`[
					{"number":83,"html_url":"https://gh/security/dependabot/83",
					 "created_at":"2026-08-19T16:34:44Z",
					 "dependency":{"package":{"name":"otel","ecosystem":"go"}},
					 "security_advisory":{"summary":"unbounded body read"},
					 "security_vulnerability":{"severity":"medium"}}
				]`))
			case "/repos/" + testRepo + "/code-scanning/alerts":
				_, _ = w.Write([]byte(`[
					{"number":12,"html_url":"https://gh/security/code-scanning/12",
					 "created_at":"2026-08-20T10:00:00Z",
					 "rule":{"id":"go/sql-injection","description":"SQL injection",
					         "security_severity_level":"high"},
					 "most_recent_instance":{"location":{"path":"api/foo.go","start_line":42}}}
				]`))
			case "/repos/" + testRepo + "/secret-scanning/alerts":
				_, _ = w.Write([]byte(`[
					{"number":7,"html_url":"https://gh/security/secret-scanning/7",
					 "created_at":"2026-08-21T09:00:00Z",
					 "secret_type_display_name":"AWS Access Key"}
				]`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	defer cleanup()

	alerts, err := newClient().ListSecurityAlerts(context.Background())
	require.NoError(t, err)
	require.Len(t, alerts, 3)

	dependabot := alerts[0]
	assert.Equal(t, github.SecurityAlertTypeDependabot, dependabot.Type)
	assert.Equal(t, int64(83), dependabot.Number)
	assert.Equal(t, "otel", dependabot.PackageName)
	assert.Equal(t, "go", dependabot.Ecosystem)
	assert.Equal(t, "medium", dependabot.Severity)
	assert.Equal(t, "unbounded body read", dependabot.Summary)
	assert.Equal(t, "https://gh/security/dependabot/83", dependabot.URL)

	codeScanning := alerts[1]
	assert.Equal(t, github.SecurityAlertTypeCodeScanning, codeScanning.Type)
	assert.Equal(t, int64(12), codeScanning.Number)
	assert.Equal(t, "go/sql-injection", codeScanning.RuleID)
	assert.Equal(t, "SQL injection", codeScanning.Summary)
	assert.Equal(t, "high", codeScanning.Severity)
	assert.Equal(t, "api/foo.go", codeScanning.FilePath)
	assert.Equal(t, int64(42), codeScanning.Line)

	secretScanning := alerts[2]
	assert.Equal(t, github.SecurityAlertTypeSecretScanning, secretScanning.Type)
	assert.Equal(t, int64(7), secretScanning.Number)
	assert.Equal(t, "AWS Access Key", secretScanning.SecretTypeDisplayName)
}

func TestListSecurityAlerts_GHASNotEnabled_OnlyDependabot(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/"+testRepo+"/dependabot/alerts" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[
					{"number":1,"html_url":"https://gh/security/dependabot/1",
					 "created_at":"2026-08-19T16:34:44Z",
					 "dependency":{"package":{"name":"lodash","ecosystem":"npm"}},
					 "security_advisory":{"summary":"prototype pollution"},
					 "security_vulnerability":{"severity":"high"}}
				]`))
				return
			}
			// code-scanning/secret-scanning 404 when GHAS isn't enabled.
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"no analysis found"}`))
		}))
	defer cleanup()

	alerts, err := newClient().ListSecurityAlerts(context.Background())
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	assert.Equal(t, github.SecurityAlertTypeDependabot, alerts[0].Type)
}

func TestListSecurityAlerts_NotConfigured_NoConnection(t *testing.T) {
	// resolveRepo fails before tokenFn is ever consulted, so the token value
	// here is irrelevant — a distinct literal from newClient()'s just avoids
	// an unparam false positive on stubToken.
	c := github.New(logging.NewNopLogger(), stubToken("unused"), configNotConnected())
	_, err := c.ListSecurityAlerts(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestListSecurityAlerts_CachesResult(t *testing.T) {
	requests := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/repos/"+testRepo+"/dependabot/alerts" {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
	defer cleanup()

	c := newClient()
	_, err := c.ListSecurityAlerts(context.Background())
	require.NoError(t, err)
	_, err = c.ListSecurityAlerts(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, requests, "second call must be served from cache")
}

func TestListSecurityAlerts_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
	defer cleanup()

	_, err := newClient().ListSecurityAlerts(context.Background())
	require.Error(t, err)
	assert.Equal(t, 4, attempts, "5xx must retry up to maxAttempts")
}

func TestListWorkflowRuns_ComputesDurationForCompletedRuns(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Query().Get("event") {
			case "pull_request":
				_, _ = w.Write([]byte(`{"workflow_runs":[
					{"id":1,"name":"CI","event":"pull_request","head_branch":"feat",
					 "status":"completed","conclusion":"success",
					 "html_url":"https://gh/run/1",
					 "run_started_at":"2026-07-01T10:00:00Z",
					 "updated_at":"2026-07-01T10:05:00Z"}
				]}`))
			case "push":
				_, _ = w.Write([]byte(`{"workflow_runs":[
					{"id":2,"name":"CI","event":"push","head_branch":"main",
					 "status":"in_progress","conclusion":"",
					 "html_url":"https://gh/run/2",
					 "run_started_at":"2026-07-01T11:00:00Z",
					 "updated_at":"2026-07-01T11:00:00Z"}
				]}`))
			}
		}))
	defer cleanup()

	runs, err := newClient().ListWorkflowRuns(context.Background())
	require.NoError(t, err)
	require.Len(t, runs, 2)

	assert.Equal(t, "pull_request", runs[0].Event)
	assert.Equal(t, int64(5*60*1000), runs[0].DurationMs)

	assert.Equal(t, "push", runs[1].Event)
	assert.Equal(t, "in_progress", runs[1].Status)
	assert.Equal(t, int64(0), runs[1].DurationMs)
}

func TestListWorkflowRuns_NotConfigured_NoConnection(t *testing.T) {
	c := github.New(logging.NewNopLogger(), stubToken("unused"), configNotConnected())
	_, err := c.ListWorkflowRuns(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestListWorkflowRuns_CachesResult(t *testing.T) {
	requests := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"workflow_runs":[]}`))
		}))
	defer cleanup()

	c := newClient()
	_, err := c.ListWorkflowRuns(context.Background())
	require.NoError(t, err)
	_, err = c.ListWorkflowRuns(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, requests, "second call must be served from cache")
}

func jsonHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}
