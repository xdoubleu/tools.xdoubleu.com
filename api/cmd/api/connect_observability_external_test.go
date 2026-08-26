package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/feeds"
	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/sentryapi"
)

// stubFeedsApp lets GetUnhealthyFeeds tests control the returned feeds
// without depending on the real feeds app/database.
type stubFeedsApp struct {
	feeds []feeds.UnhealthyFeed
	err   error
}

func (s stubFeedsApp) ListUnhealthy(
	_ context.Context,
) ([]feeds.UnhealthyFeed, error) {
	return s.feeds, s.err
}

// withFeedsApp swaps testApp's feeds app for a stub for the duration of the
// test.
func withFeedsApp(t *testing.T, app unhealthyFeedLister) {
	t.Helper()
	orig := testApp.feedsApp
	testApp.feedsApp = app
	t.Cleanup(func() { testApp.feedsApp = orig })
}

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
					 "user":{"login":"alice"},"head":{"sha":"sha1"},
					 "labels":[{"name":"dependencies"}]}
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

// --- Workflow runs ---

func TestObservabilityGetWorkflowRuns_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Query().Get("event") {
			case "pull_request":
				_, _ = w.Write([]byte(`{"workflow_runs":[
					{"id":1,"name":"CI","event":"pull_request","head_branch":"feat",
					 "status":"completed","conclusion":"success","html_url":"u1",
					 "run_started_at":"2026-07-01T10:00:00Z",
					 "updated_at":"2026-07-01T10:05:00Z"}
				]}`))
			case "push":
				_, _ = w.Write([]byte(`{"workflow_runs":[]}`))
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

	resp, err := callWorkflowRuns(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.Runs, 1)
	assert.Equal(t, "pull_request", resp.Msg.Runs[0].Event)
	assert.Equal(t, int64(5*60*1000), resp.Msg.Runs[0].DurationMs)
}

func TestObservabilityGetWorkflowRuns_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	resp, err := callWorkflowRuns(t)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Runs)
}

func TestObservabilityGetWorkflowRuns_UpstreamError(t *testing.T) {
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

	resp, err := callWorkflowRuns(t)
	require.NoError(t, err) // degraded, never a failed response
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Runs)
}

func TestObservabilityGetWorkflowRuns_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callWorkflowRuns(t)
	requirePermissionDenied(t, err)
}

func callWorkflowRuns(
	t *testing.T,
) (*connect.Response[observabilityv1.GetWorkflowRunsResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetWorkflowRunsRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetWorkflowRuns(context.Background(), req)
}

// --- Workflow run stats ---

func TestObservabilityGetWorkflowRunStats_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	resp, err := callWorkflowRunStats(t)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}

// TestObservabilityGetWorkflowRunStats_ReportsRecordedHistory seeds real
// run/job samples so the aggregation loops (main-branch failures, per-
// workflow and per-job duration stats) actually run over non-empty data.
func TestObservabilityGetWorkflowRunStats_ReportsRecordedHistory(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	ctx := context.Background()
	_, _ = testApp.db.Exec(ctx, "DELETE FROM global.workflow_job_samples")
	_, _ = testApp.db.Exec(ctx, "DELETE FROM global.workflow_run_samples")
	t.Cleanup(func() {
		_, _ = testApp.db.Exec(ctx, "DELETE FROM global.workflow_job_samples")
		_, _ = testApp.db.Exec(ctx, "DELETE FROM global.workflow_run_samples")
	})

	now := time.Now()
	require.NoError(t, testApp.workflowRunsRepo.InsertRun(ctx, models.WorkflowRunSample{
		RunID: 9001, WorkflowName: "CI", Branch: "main", Event: "push",
		Conclusion: "failure", URL: "https://github.com/x/y/actions/runs/9001",
		DurationMs: 60_000, StartedAt: now.Add(-time.Minute), CompletedAt: now,
	}))
	require.NoError(t, testApp.workflowRunsRepo.InsertJobs(
		ctx, []models.WorkflowJobSample{
			{
				RunID: 9001, JobName: "build", Conclusion: "failure",
				DurationMs: 30_000, StartedAt: now.Add(-time.Minute), CompletedAt: now,
			},
		},
	))

	resp, err := callWorkflowRunStats(t)
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetMainFailures(), 1)
	assert.Equal(t, int64(9001), resp.Msg.GetMainFailures()[0].GetRunId())
	require.Len(t, resp.Msg.GetWorkflowDurationStats(), 1)
	assert.Equal(t, "CI", resp.Msg.GetWorkflowDurationStats()[0].GetWorkflowName())
	require.Len(t, resp.Msg.GetJobDurationStats(), 1)
	assert.Equal(t, "build", resp.Msg.GetJobDurationStats()[0].GetJobName())
}

func TestObservabilityGetWorkflowRunStats_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callWorkflowRunStats(t)
	requirePermissionDenied(t, err)
}

// TestObservabilityWorkflowRunStats_QueryErrorPropagates exercises
// workflowRunStats' error-propagation branches directly (bypassing the
// Connect handler) by passing an already-canceled context, so the
// underlying repository query fails immediately.
func TestObservabilityWorkflowRunStats_QueryErrorPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h := &obsConnectHandler{app: testApp}
	_, err := h.workflowRunStats(ctx, 0)
	require.Error(t, err)
}

func callWorkflowRunStats(
	t *testing.T,
) (*connect.Response[observabilityv1.GetWorkflowRunStatsResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetWorkflowRunStatsRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetWorkflowRunStats(context.Background(), req)
}

// --- Security alerts ---

func TestObservabilityGetSecurityAlerts_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/o/r/dependabot/alerts":
				_, _ = w.Write([]byte(`[
					{"number":83,"html_url":"u",
					 "created_at":"2026-08-19T16:34:44Z",
					 "dependency":{"package":{"name":"otel","ecosystem":"go"}},
					 "security_advisory":{"summary":"unbounded body read"},
					 "security_vulnerability":{"severity":"medium"}}
				]`))
			case "/repos/o/r/code-scanning/alerts":
				_, _ = w.Write([]byte(`[
					{"number":12,"html_url":"u2",
					 "created_at":"2026-08-20T10:00:00Z",
					 "rule":{"id":"go/sql-injection","description":"SQL injection",
					         "security_severity_level":"high"},
					 "most_recent_instance":{"location":{"path":"api/foo.go","start_line":42}}}
				]`))
			case "/repos/o/r/secret-scanning/alerts":
				_, _ = w.Write([]byte(`[
					{"number":7,"html_url":"u3",
					 "created_at":"2026-08-21T08:00:00Z",
					 "secret_type_display_name":"AWS Access Key"}
				]`))
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

	resp, err := callSecurityAlerts(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.Alerts, 3)
	assert.Equal(t, int64(83), resp.Msg.Alerts[0].Number)
	assert.Equal(t, "otel", resp.Msg.Alerts[0].PackageName)
	assert.Equal(t, "medium", resp.Msg.Alerts[0].Severity)
	assert.Equal(
		t,
		observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_DEPENDABOT,
		resp.Msg.Alerts[0].AlertType,
	)

	assert.Equal(t, int64(12), resp.Msg.Alerts[1].Number)
	assert.Equal(t, "go/sql-injection", resp.Msg.Alerts[1].RuleId)
	assert.Equal(t, "api/foo.go", resp.Msg.Alerts[1].FilePath)
	assert.Equal(t, int32(42), resp.Msg.Alerts[1].Line)
	assert.Equal(
		t,
		observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_CODE_SCANNING,
		resp.Msg.Alerts[1].AlertType,
	)
	assert.Equal(t, int64(7), resp.Msg.Alerts[2].Number)
	assert.Equal(t, "AWS Access Key", resp.Msg.Alerts[2].SecretType)
	assert.Equal(
		t,
		observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_SECRET_SCANNING,
		resp.Msg.Alerts[2].AlertType,
	)
	assert.Equal(t, int32(3), resp.Msg.AlertCount)
}

func TestObservabilityGetSecurityAlerts_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	resp, err := callSecurityAlerts(t)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Alerts)
}

func TestObservabilityGetSecurityAlerts_UpstreamError(t *testing.T) {
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

	resp, err := callSecurityAlerts(t)
	require.NoError(t, err) // degraded, never a failed response
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Alerts)
}

func TestObservabilityGetSecurityAlerts_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callSecurityAlerts(t)
	requirePermissionDenied(t, err)
}

func callSecurityAlerts(
	t *testing.T,
) (*connect.Response[observabilityv1.GetSecurityAlertsResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetSecurityAlertsRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetSecurityAlerts(context.Background(), req)
}

// --- Unhealthy feeds ---

func TestObservabilityGetUnhealthyFeeds_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	withFeedsApp(t, stubFeedsApp{
		feeds: []feeds.UnhealthyFeed{
			{
				Title:               "Broken Feed",
				URL:                 "https://example.com/feed.xml",
				LastError:           "timeout",
				ConsecutiveFailures: 3,
			},
		},
		err: nil,
	})

	resp, err := callUnhealthyFeeds(t)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Feeds, 1)
	assert.Equal(t, "Broken Feed", resp.Msg.Feeds[0].Title)
	assert.Equal(t, "https://example.com/feed.xml", resp.Msg.Feeds[0].Url)
	assert.Equal(t, "timeout", resp.Msg.Feeds[0].LastError)
	assert.Equal(t, int32(3), resp.Msg.Feeds[0].ConsecutiveFailures)
}

func TestObservabilityGetUnhealthyFeeds_Empty(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	withFeedsApp(t, stubFeedsApp{feeds: nil, err: nil})

	resp, err := callUnhealthyFeeds(t)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Feeds)
}

func TestObservabilityGetUnhealthyFeeds_Error(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	withFeedsApp(t, stubFeedsApp{feeds: nil, err: errors.New("db down")})

	_, err := callUnhealthyFeeds(t)
	require.Error(t, err)
}

func TestObservabilityGetUnhealthyFeeds_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callUnhealthyFeeds(t)
	requirePermissionDenied(t, err)
}

func callUnhealthyFeeds(
	t *testing.T,
) (*connect.Response[observabilityv1.GetUnhealthyFeedsResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetUnhealthyFeedsRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetUnhealthyFeeds(context.Background(), req)
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

func TestObservabilityResolveSentryIssue_ReauthRequired(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	// Config resolves (a connection+config row exists) but the token func
	// reports ErrNotConnected — i.e. a stale granted scope, not "never
	// connected" — which must surface as CodeUnauthenticated, not
	// CodeFailedPrecondition (issue #791).
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(),
		stubTok(""),
		testConfigJSON(t, map[string]any{"org": "org", "projects": []string{"proj"}}),
	)

	_, err := callResolveSentryIssue(t)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
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

// --- Health overview (mixed states) ---

func TestObservabilityGetHealthOverview_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	// Sentry configured but upstream fails — the section degrades
	// independently.
	se := jsonServer(t, http.StatusInternalServerError, ``)
	sentryapi.SetBaseURL(se.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"),
		testConfigJSON(t, map[string]any{"org": "org", "projects": []string{"proj"}}),
	)

	req := connect.NewRequest(&observabilityv1.GetHealthOverviewRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).GetHealthOverview(
		context.Background(), req,
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Sentry.Configured) // configured, upstream failed
	assert.Empty(t, resp.Msg.Sentry.Issues)
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

// --- Slow transactions ---

func clearTransactionLatencyDaily(t *testing.T) {
	t.Helper()
	_, err := testApp.db.Exec(
		context.Background(), "DELETE FROM global.transaction_latency_daily",
	)
	require.NoError(t, err)
}

func TestObservabilityGetSlowTransactions_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearTransactionLatencyDaily(t)

	srv := jsonServer(t, http.StatusOK, `{"data": [
		{"transaction":"GET /api/slow","project":"proj",
		 "p95(transaction.duration)":900,"count()":3},
		{"transaction":"GET /api/fast","project":"proj",
		 "p95(transaction.duration)":50,"count()":30}
	]}`)
	sentryapi.SetBaseURL(srv.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"),
		testConfigJSON(t, map[string]any{"org": "org", "projects": []string{"proj"}}),
	)

	resp, err := callSlowTransactions(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.Current, 2)
	assert.Equal(t, "GET /api/slow", resp.Msg.Current[0].Transaction, "slowest first")
	assert.Equal(t, "GET /api/fast", resp.Msg.Current[1].Transaction)
	assert.Empty(t, resp.Msg.Trending, "no history seeded yet")
}

func TestObservabilityGetSlowTransactions_NotConfiguredStillReportsTrending(
	t *testing.T,
) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearTransactionLatencyDaily(t)

	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), stubTok("tok"), configNotConnected(),
	)

	now := time.Now()
	seedTransactionLatencyDay(t, now.Add(-3*24*time.Hour), "GET /api/regressed", 300)
	seedTransactionLatencyDay(t, now.Add(-10*24*time.Hour), "GET /api/regressed", 100)

	resp, err := callSlowTransactions(t)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Current)
	require.Len(t, resp.Msg.Trending, 1,
		"trending must still be reported when Sentry itself is unconfigured")
	assert.Equal(t, "GET /api/regressed", resp.Msg.Trending[0].Transaction)
	assert.InEpsilon(t, 2.0, resp.Msg.Trending[0].PctChange, 0.001)
}

func TestObservabilityGetSlowTransactions_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callSlowTransactions(t)
	requirePermissionDenied(t, err)
}

func seedTransactionLatencyDay(
	t *testing.T,
	day time.Time,
	transaction string,
	p95Ms float64,
) {
	t.Helper()
	_, err := testApp.db.Exec(context.Background(), `
		INSERT INTO global.transaction_latency_daily
			(day, project, transaction_name, p95_duration_ms, request_count)
		VALUES ($1, 'proj', $2, $3, 1)
	`, day, transaction, p95Ms)
	require.NoError(t, err)
}

func callSlowTransactions(
	t *testing.T,
) (*connect.Response[observabilityv1.GetSlowTransactionsResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetSlowTransactionsRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetSlowTransactions(context.Background(), req)
}

// --- Host metrics ---

func TestObservabilityGetHostMetrics_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	_, err := testApp.db.Exec(
		context.Background(), "DELETE FROM global.host_metric_samples",
	)
	require.NoError(t, err)
	_, err = testApp.db.Exec(context.Background(), `
		INSERT INTO global.host_metric_samples
			(sampled_at, cpu_percent, memory_percent, disk_percent)
		VALUES (now(), 12.5, 40.0, 60.0)
	`)
	require.NoError(t, err)

	req := connect.NewRequest(&observabilityv1.GetHostMetricsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).GetHostMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.InDelta(t, 12.5, resp.Msg.CpuPercent, 0.001)
	require.NotEmpty(t, resp.Msg.CpuHistory)
}

func TestObservabilityGetHostMetrics_NonAdmin(t *testing.T) {
	demoteToUser(t)
	req := connect.NewRequest(&observabilityv1.GetHostMetricsRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).GetHostMetrics(context.Background(), req)
	requirePermissionDenied(t, err)
}

// --- Logs ---

func TestObservabilityGetLogs_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	_, err := testApp.db.Exec(
		context.Background(), "DELETE FROM global.log_entries",
	)
	require.NoError(t, err)
	_, err = testApp.db.Exec(context.Background(), `
		INSERT INTO global.log_entries (occurred_at, source, level, message)
		VALUES (now(), 'api', 'info', 'hello from a test')
	`)
	require.NoError(t, err)

	req := connect.NewRequest(&observabilityv1.GetLogsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).GetLogs(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Entries, 1)
	assert.Equal(t, "hello from a test", resp.Msg.Entries[0].Message)
}

func TestObservabilityGetLogs_NonAdmin(t *testing.T) {
	demoteToUser(t)
	req := connect.NewRequest(&observabilityv1.GetLogsRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).GetLogs(context.Background(), req)
	requirePermissionDenied(t, err)
}
