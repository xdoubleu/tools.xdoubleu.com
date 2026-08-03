package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"golang.org/x/oauth2"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/sentryapi"
)

func stubTok(token string) oauthconn.TokenFunc {
	return func(context.Context) (string, error) {
		if token == "" {
			return "", oauthconn.ErrNotConnected
		}
		return token, nil
	}
}

// stubConfigStore stands in for *repositories.OAuthConnectionsRepository in
// tests that build a provider client directly (bypassing newObservabilityClients).
type stubConfigStore struct {
	conn *models.OAuthConnection
	err  error
}

func (s stubConfigStore) Get(
	context.Context, models.OAuthProvider,
) (*oauth2.Token, *models.OAuthConnection, error) {
	return nil, s.conn, s.err
}

func testConfigJSON(t *testing.T, v any) stubConfigStore {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return stubConfigStore{
		conn: &models.OAuthConnection{Config: raw}, //nolint:exhaustruct // test fixture
		err:  nil,
	}
}

func configNotConnected() stubConfigStore {
	//nolint:exhaustruct // conn intentionally nil: simulates "not connected"
	return stubConfigStore{err: database.ErrResourceNotFound}
}

// jsonServer starts an httptest server returning status/body and registers its
// cleanup. Retries are sped up so upstream-error tests don't sleep.
func jsonServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	github.SetBackoffBase(time.Millisecond)
	sentryapi.SetBackoffBase(time.Millisecond)
	digitalocean.SetBackoffBase(time.Millisecond)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
	t.Cleanup(srv.Close)
	return srv
}

// --- Failing pull requests ---

func TestObservabilityGetFailingPullRequests_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/o/r/pulls":
				_, _ = w.Write([]byte(`[
					{"number":3,"title":"Broken","html_url":"u",
					 "updated_at":"2026-07-01T00:00:00Z",
					 "user":{"login":"alice"},"head":{"sha":"sha1"}}
				]`))
			case "/repos/o/r/commits/sha1/check-runs":
				_, _ = w.Write([]byte(`{"check_runs":[
					{"name":"ci-pass","status":"completed","conclusion":"failure",
					 "html_url":"u"}
				]}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	t.Cleanup(srv.Close)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	resp, err := callFailingPullRequests(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.PullRequests, 1)
	assert.Equal(t, int64(3), resp.Msg.PullRequests[0].Number)
	assert.Equal(t, "alice", resp.Msg.PullRequests[0].Author)
	require.Len(t, resp.Msg.PullRequests[0].FailingChecks, 1)
	assert.Equal(t, int32(1), resp.Msg.FailingCount)
}

func TestObservabilityGetFailingPullRequests_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	resp, err := callFailingPullRequests(t)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.PullRequests)
}

func TestObservabilityGetFailingPullRequests_UpstreamError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusInternalServerError, ``)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	resp, err := callFailingPullRequests(t)
	require.NoError(t, err) // degraded, never a failed response
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.PullRequests)
}

func TestObservabilityGetFailingPullRequests_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callFailingPullRequests(t)
	requirePermissionDenied(t, err)
}

func callFailingPullRequests(
	t *testing.T,
) (*connect.Response[observabilityv1.GetFailingPullRequestsResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetFailingPullRequestsRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetFailingPullRequests(context.Background(), req)
}

// --- Sentry ---

func TestObservabilityGetSentryIssues_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusOK, `[
		{"id":"42","title":"boom","culprit":"main.go","permalink":"pl",
		 "count":"9","lastSeen":"2026-07-10T00:00:00Z","level":"error"}
	]`)
	sentryapi.SetBaseURL(srv.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"),
		testConfigJSON(t, map[string]any{"org": "org", "projects": []string{"proj"}}),
	)

	resp, err := callSentry(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.Issues, 1)
	assert.Equal(t, "42", resp.Msg.Issues[0].Id)
	assert.Equal(t, int64(9), resp.Msg.Issues[0].Count)
	assert.Equal(t, int32(1), resp.Msg.UnresolvedCount)
}

func TestObservabilityGetSentryIssues_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	resp, err := callSentry(t)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Issues)
}

func TestObservabilityGetSentryIssues_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callSentry(t)
	requirePermissionDenied(t, err)
}

func callSentry(
	t *testing.T,
) (*connect.Response[observabilityv1.GetSentryIssuesResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetSentryIssuesRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetSentryIssues(context.Background(), req)
}

func TestObservabilityResolveSentryIssue_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
	t.Cleanup(srv.Close)
	sentryapi.SetBaseURL(srv.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"),
		testConfigJSON(t, map[string]any{"org": "org", "projects": []string{"proj"}}),
	)

	_, err := callResolveSentryIssue(t)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, gotMethod)
	assert.True(t, strings.HasSuffix(gotPath, "/api/0/issues/42/"))
}

func TestObservabilityResolveSentryIssue_UpstreamError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusInternalServerError, ``)
	sentryapi.SetBaseURL(srv.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"),
		testConfigJSON(t, map[string]any{"org": "org", "projects": []string{"proj"}}),
	)

	_, err := callResolveSentryIssue(t)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestObservabilityResolveSentryIssue_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	_, err := callResolveSentryIssue(t)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestObservabilityResolveSentryIssue_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callResolveSentryIssue(t)
	requirePermissionDenied(t, err)
}

func callResolveSentryIssue(
	t *testing.T,
) (*connect.Response[observabilityv1.ResolveSentryIssueResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.ResolveSentryIssueRequest{
		IssueId: "42",
	})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).ResolveSentryIssue(context.Background(), req)
}

// --- Deploy status ---

func TestObservabilityGetDeployStatus_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusOK, `{"deployments":[
		{"id":"d1","phase":"ACTIVE","cause":"push",
		 "created_at":"2026-07-10T00:00:00Z","updated_at":"2026-07-10T00:05:00Z"}
	]}`)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	resp, err := callDeploy(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	assert.Equal(t, "ACTIVE", resp.Msg.Phase)
	assert.Equal(t, "d1", resp.Msg.DeploymentId)
}

func TestObservabilityGetDeployStatus_NoDeployment(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusOK, `{"deployments":[]}`)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	resp, err := callDeploy(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Phase)
}

func TestObservabilityGetDeployStatus_UpstreamError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusBadGateway, ``)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	resp, err := callDeploy(t)
	require.NoError(t, err) // degraded, never a failed response
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Phase)
}

func TestObservabilityGetDeployStatus_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	resp, err := callDeploy(t)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
}

func TestObservabilityGetDeployStatus_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callDeploy(t)
	requirePermissionDenied(t, err)
}

func callDeploy(
	t *testing.T,
) (*connect.Response[observabilityv1.GetDeployStatusResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetDeployStatusRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetDeployStatus(context.Background(), req)
}

// --- Deploy logs ---

// deployLogsMux builds a fake DigitalOcean server serving the deployments
// list (for LatestDeployment), one deployment's detail (service component
// names), its /logs endpoint, and the log-chunk content those URLs point at.
func deployLogsMux(latestID string, components []string, chunk string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/v2/apps/app/deployments",
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"deployments":[{"id":"` + latestID + `","phase":"ACTIVE"}]}`,
			))
		},
	)
	names := make([]string, len(components))
	for i, c := range components {
		names[i] = `{"name":"` + c + `"}`
	}
	mux.HandleFunc(
		"/v2/apps/app/deployments/"+latestID,
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"deployment":{"services":[` + strings.Join(
					names,
					",",
				) + `]}}`,
			))
		},
	)
	mux.HandleFunc(
		"/v2/apps/app/deployments/"+latestID+"/logs",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Query().Get("type") != "BUILD" {
				_, _ = w.Write([]byte(`{"historic_urls":[]}`))
				return
			}
			_, _ = w.Write([]byte(
				`{"historic_urls":["http://` + r.Host + `/log-chunk"]}`,
			))
		},
	)
	mux.HandleFunc("/v2/apps/app", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"app":{"active_deployment":{"id":"` + latestID + `","services":[` +
				strings.Join(names, ",") + `]}}}`,
		))
	})
	mux.HandleFunc("/log-chunk", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(chunk))
	})
	return mux
}

func TestObservabilityGetDeployLogs_LatestDeployment(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	digitalocean.SetBackoffBase(time.Millisecond)

	srv := httptest.NewServer(deployLogsMux("d1", []string{"api"}, "building\n"))
	t.Cleanup(srv.Close)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	logs, err := callDeployLogs(t, "", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "api", logs[0].Component)
	assert.Equal(t, "BUILD", logs[0].LogType)
	assert.Equal(t, "d1", logs[0].DeploymentId)
	assert.Equal(t, "building\n", logs[0].Content)
	assert.False(t, logs[0].Truncated)
}

func TestObservabilityGetDeployLogs_ExplicitDeploymentId(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	digitalocean.SetBackoffBase(time.Millisecond)

	srv := httptest.NewServer(deployLogsMux("d2", []string{"web"}, "deploying\n"))
	t.Cleanup(srv.Close)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	logs, err := callDeployLogs(t, "d2", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "web", logs[0].Component)
	assert.Equal(t, "d2", logs[0].DeploymentId)
}

// tail_lines has to reach DigitalOcean, and every block has to name the
// deployment it came from — runtime logs may come from the active deployment
// rather than the requested one.
func TestObservabilityGetDeployLogs_ForwardsTailLines(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	digitalocean.SetBackoffBase(time.Millisecond)

	var seen string
	mux := deployLogsMux("d1", []string{"app"}, "building\n")
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v2/apps/app/deployments/d1/logs" {
				seen = r.URL.Query().Get("tail_lines")
			}
			mux.ServeHTTP(w, r)
		}))
	t.Cleanup(srv.Close)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	logs, err := callDeployLogs(t, "", 250)
	require.NoError(t, err)
	assert.Equal(t, "250", seen)
	require.Len(t, logs, 1)
	assert.Equal(t, "d1", logs[0].DeploymentId)
}

func TestObservabilityGetDeployLogs_UpstreamError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusBadGateway, ``)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	logs, err := callDeployLogs(t, "d1", 0)
	require.NoError(t, err) // degraded, never a failed response
	assert.Empty(t, logs)
}

func TestObservabilityGetDeployLogs_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	logs, err := callDeployLogs(t, "", 0)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestObservabilityGetDeployLogs_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callDeployLogs(t, "", 0)
	requirePermissionDenied(t, err)
}

// callDeployLogs drains the GetDeployLogs stream and returns every message
// it sent, in receive order (not request order — components resolve
// concurrently). err is the stream's terminal error, e.g. permission denied
// for a non-admin caller.
func callDeployLogs(
	t *testing.T, deploymentID string, tailLines int32,
) ([]*observabilityv1.DeployComponentLog, error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetDeployLogsRequest{
		DeploymentId: deploymentID,
		TailLines:    tailLines,
	})
	setCookieOnRequest(req, accessToken)

	stream, err := observabilityClient(t).GetDeployLogs(context.Background(), req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var logs []*observabilityv1.DeployComponentLog
	for stream.Receive() {
		logs = append(logs, stream.Msg().GetLog())
	}
	return logs, stream.Err()
}

// --- Deploy logs (unary, MCP path) ---
//
// deployLogs/resolveLatestDeploymentID/degradeDeployLogs back the
// get_deploy_logs MCP tool directly (mcp_apps.go), not through Connect, so
// they need their own coverage now that GetDeployLogs itself streams.

func TestDeployLogsUnary_LatestDeployment(t *testing.T) {
	digitalocean.SetBackoffBase(time.Millisecond)
	srv := httptest.NewServer(deployLogsMux("d1", []string{"worker"}, "building\n"))
	t.Cleanup(srv.Close)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })

	h := &obsConnectHandler{app: testApp}
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	resp := h.deployLogs(context.Background(), "", 0)
	assert.True(t, resp.GetConfigured())
	assert.Equal(t, "d1", resp.GetDeploymentId())
	require.Len(t, resp.GetLogs(), 1)
	assert.Equal(t, "worker", resp.GetLogs()[0].GetComponent())
	assert.Equal(t, "building\n", resp.GetLogs()[0].GetContent())
}

func TestDeployLogsUnary_NotConfigured(t *testing.T) {
	h := &obsConnectHandler{app: testApp}
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok(""),
		configNotConnected(),
	)

	resp := h.deployLogs(context.Background(), "", 0)
	assert.False(t, resp.GetConfigured())
	assert.Empty(t, resp.GetLogs())
}

func TestDeployLogsUnary_UpstreamErrorDegrades(t *testing.T) {
	srv := jsonServer(t, http.StatusBadGateway, ``)
	digitalocean.SetBaseURL(srv.URL)
	t.Cleanup(func() { digitalocean.SetBaseURL("https://api.digitalocean.com") })

	h := &obsConnectHandler{app: testApp}
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"app_id": "app"}),
	)

	resp := h.deployLogs(context.Background(), "d1", 0)
	assert.True(t, resp.GetConfigured()) // degraded, never a failed response
	assert.Empty(t, resp.GetLogs())
}

// --- Health overview (mixed states) ---

func TestObservabilityGetHealthOverview_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	// Sentry configured but upstream fails; deploy unconfigured — each
	// section degrades independently.
	se := jsonServer(t, http.StatusInternalServerError, ``)
	sentryapi.SetBaseURL(se.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"),
		testConfigJSON(t, map[string]any{"org": "org", "projects": []string{"proj"}}),
	)

	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	req := connect.NewRequest(&observabilityv1.GetHealthOverviewRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).GetHealthOverview(
		context.Background(), req,
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Sentry.Configured) // configured, upstream failed
	assert.Empty(t, resp.Msg.Sentry.Issues)
	assert.False(t, resp.Msg.Deploy.Configured)
}

func TestObservabilityGetHealthOverview_NonAdmin(t *testing.T) {
	demoteToUser(t)
	req := connect.NewRequest(&observabilityv1.GetHealthOverviewRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).GetHealthOverview(
		context.Background(), req,
	)
	requirePermissionDenied(t, err)
}
