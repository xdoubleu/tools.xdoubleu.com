package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/sentryapi"
)

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

// --- GitHub ---

func TestObservabilityGetGithubIssues_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusOK, `[
		{"number":7,"title":"Bug","html_url":"u","state":"open",
		 "created_at":"2026-07-01T00:00:00Z","labels":[{"name":"bug"}]},
		{"number":8,"title":"PR","html_url":"p","pull_request":{"url":"x"}}
	]`)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(logging.NewNopLogger(), "tok", "o/r")

	resp, err := callGithub(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.Issues, 1)
	assert.Equal(t, int64(7), resp.Msg.Issues[0].Number)
	assert.Equal(t, int32(1), resp.Msg.OpenCount)
}

func TestObservabilityGetGithubIssues_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(logging.NewNopLogger(), "", "")

	resp, err := callGithub(t)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Issues)
}

func TestObservabilityGetGithubIssues_UpstreamError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusInternalServerError, ``)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(logging.NewNopLogger(), "tok", "o/r")

	resp, err := callGithub(t)
	require.NoError(t, err) // degraded, never a failed response
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Issues)
}

func TestObservabilityGetGithubIssues_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callGithub(t)
	requirePermissionDenied(t, err)
}

func callGithub(
	t *testing.T,
) (*connect.Response[observabilityv1.GetGithubIssuesResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetGithubIssuesRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetGithubIssues(context.Background(), req)
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
		logging.NewNopLogger(), "org", "proj", "tok",
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
	testApp.sentryClient = sentryapi.New(logging.NewNopLogger(), "", "", "")

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
	testApp.doClient = digitalocean.New(logging.NewNopLogger(), "tok", "app")

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
	testApp.doClient = digitalocean.New(logging.NewNopLogger(), "tok", "app")

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
	testApp.doClient = digitalocean.New(logging.NewNopLogger(), "tok", "app")

	resp, err := callDeploy(t)
	require.NoError(t, err) // degraded, never a failed response
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Phase)
}

func TestObservabilityGetDeployStatus_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.doClient = digitalocean.New(logging.NewNopLogger(), "", "")

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

// --- Health overview (mixed states) ---

func TestObservabilityGetHealthOverview_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	// GitHub configured & healthy; Sentry configured but upstream fails;
	// deploy unconfigured — each section degrades independently.
	gh := jsonServer(t, http.StatusOK,
		`[{"number":1,"title":"x","html_url":"u","state":"open"}]`)
	github.SetBaseURL(gh.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(logging.NewNopLogger(), "tok", "o/r")

	se := jsonServer(t, http.StatusInternalServerError, ``)
	sentryapi.SetBaseURL(se.URL)
	t.Cleanup(func() { sentryapi.SetBaseURL("https://sentry.io") })
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(), "org", "proj", "tok",
	)

	testApp.doClient = digitalocean.New(logging.NewNopLogger(), "", "")

	req := connect.NewRequest(&observabilityv1.GetHealthOverviewRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).GetHealthOverview(
		context.Background(), req,
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Github.Configured)
	assert.Len(t, resp.Msg.Github.Issues, 1)
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
