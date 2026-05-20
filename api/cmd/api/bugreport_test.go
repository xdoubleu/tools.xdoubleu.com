package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/cmd/api/internal/logging"
	bugreportv1 "tools.xdoubleu.com/gen/bugreport/v1"
	"tools.xdoubleu.com/gen/bugreport/v1/bugreportv1connect"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(
	r *http.Request,
) (*http.Response, error) {
	return f(r)
}

func bugReportClient(t *testing.T) bugreportv1connect.BugReportServiceClient {
	t.Helper()
	ts := connectServer(t)
	return bugreportv1connect.NewBugReportServiceClient(ts.Client(), ts.URL)
}

func TestCreateBugReportNotConfigured(t *testing.T) {
	client := bugReportClient(t)
	req := connect.NewRequest(&bugreportv1.CreateBugReportRequest{
		Title:       "Test bug",
		Description: "Something broke",
		Page:        "",
		ConsoleLogs: "",
		WsLog:       "",
	})
	setCookieOnRequest(req, accessToken)

	_, err := client.CreateBugReport(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnavailable, connectErr.Code())
}

func TestCreateBugReportUnauthorized(t *testing.T) {
	client := bugReportClient(t)
	_, err := client.CreateBugReport(
		context.Background(),
		connect.NewRequest(&bugreportv1.CreateBugReportRequest{
			Title:       "Test bug",
			Description: "Something broke",
			Page:        "",
			ConsoleLogs: "",
			WsLog:       "",
		}),
	)
	require.Error(t, err)
}

func TestCreateBugReportEmptyFields(t *testing.T) {
	// Use testAppWithGitHub which has GitHub configured
	ts := httptest.NewServer(testAppWithGitHub.Routes())
	defer ts.Close()
	client := bugreportv1connect.NewBugReportServiceClient(ts.Client(), ts.URL)

	req := connect.NewRequest(&bugreportv1.CreateBugReportRequest{
		Title:       "",
		Description: "",
		Page:        "",
		ConsoleLogs: "",
		WsLog:       "",
	})
	setCookieOnRequest(req, accessToken)

	_, err := client.CreateBugReport(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestCreateBugReportSuccess(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			require.NoError(t, json.NewEncoder(w).Encode(githubIssueResponse{
				HTMLURL: "https://github.com/owner/repo/issues/42",
			}))
		}),
	)
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			req2 := req.Clone(req.Context())
			parsed, _ := url.Parse(srv.URL)
			req2.URL.Scheme = parsed.Scheme
			req2.URL.Host = parsed.Host
			return origTransport.RoundTrip(req2)
		},
	)
	defer func() { http.DefaultTransport = origTransport }()

	ts := httptest.NewServer(testAppWithGitHub.Routes())
	defer ts.Close()
	client := bugreportv1connect.NewBugReportServiceClient(ts.Client(), ts.URL)

	req := connect.NewRequest(&bugreportv1.CreateBugReportRequest{
		Title:       "Real bug",
		Description: "Something broke on page /settings",
		Page:        "",
		ConsoleLogs: "",
		WsLog:       "",
	})
	setCookieOnRequest(req, accessToken)

	resp, err := client.CreateBugReport(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/owner/repo/issues/42", resp.Msg.Url)
}

func TestCreateBugReportGitHubError(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Validation Failed"}`))
		}),
	)
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			req2 := req.Clone(req.Context())
			parsed, _ := url.Parse(srv.URL)
			req2.URL.Scheme = parsed.Scheme
			req2.URL.Host = parsed.Host
			return origTransport.RoundTrip(req2)
		},
	)
	defer func() { http.DefaultTransport = origTransport }()

	ts := httptest.NewServer(testAppWithGitHub.Routes())
	defer ts.Close()
	client := bugreportv1connect.NewBugReportServiceClient(ts.Client(), ts.URL)

	req := connect.NewRequest(&bugreportv1.CreateBugReportRequest{
		Title:       "Real bug",
		Description: "Something broke on page /settings",
		Page:        "",
		ConsoleLogs: "",
		WsLog:       "",
	})
	setCookieOnRequest(req, accessToken)

	_, err := client.CreateBugReport(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

func TestCreateGitHubIssue(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			require.NoError(t, json.NewEncoder(w).Encode(githubIssueResponse{
				HTMLURL: "https://github.com/owner/repo/issues/1",
			}))
		}),
	)
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			req2 := req.Clone(req.Context())
			parsed, _ := url.Parse(srv.URL)
			req2.URL.Scheme = parsed.Scheme
			req2.URL.Host = parsed.Host
			return origTransport.RoundTrip(req2)
		},
	)
	defer func() { http.DefaultTransport = origTransport }()

	issueURL, err := createGitHubIssue(
		context.Background(),
		"test-token",
		"owner/repo",
		"Test Issue",
		"Test body",
	)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/owner/repo/issues/1", issueURL)
}

func TestCreateGitHubIssueNon201Response(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Validation Failed"}`))
		}),
	)
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			req2 := req.Clone(req.Context())
			parsed, _ := url.Parse(srv.URL)
			req2.URL.Scheme = parsed.Scheme
			req2.URL.Host = parsed.Host
			return origTransport.RoundTrip(req2)
		},
	)
	defer func() { http.DefaultTransport = origTransport }()

	_, err := createGitHubIssue(
		context.Background(),
		"test-token",
		"owner/repo",
		"Test",
		"Body",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "422")
}

func TestBuildIssueBodyEscapesPipes(t *testing.T) {
	entries := []logging.LogEntry{
		{
			Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Level:   "ERROR",
			Message: "failed | with pipes\nand newline",
		},
	}

	body := buildIssueBody(
		"desc",
		"https://example.com/page",
		"Mozilla/5.0",
		"v1.2.3",
		"production",
		"user-123",
		entries,
		"",
		"",
	)

	assert.Contains(t, body, `failed \| with pipes and newline`)
	assert.Contains(t, body, "https://example.com/page")
	assert.Contains(t, body, "Mozilla/5.0")
	assert.Contains(t, body, "v1.2.3")
	assert.Contains(t, body, "production")
	assert.Contains(t, body, "user-123")
}

func TestBuildIssueBody_NoEntries(t *testing.T) {
	body := buildIssueBody(
		"description", "/page", "agent", "v0.0.1", "dev", "uid",
		nil, "", "",
	)
	assert.Contains(t, body, "_No log entries captured._")
}

func TestBuildIssueBody_WithConsoleLogs(t *testing.T) {
	body := buildIssueBody(
		"description", "/page", "agent", "v0.0.1", "dev", "uid",
		nil, "console.log('hi')", "",
	)
	assert.Contains(t, body, "console.log('hi')")
	assert.Contains(t, body, "```")
}

func TestBuildIssueBody_WithWSLog(t *testing.T) {
	body := buildIssueBody(
		"description", "/page", "agent", "v0.0.1", "dev", "uid",
		nil, "", "ws message here",
	)
	assert.Contains(t, body, "ws message here")
	assert.Contains(t, body, "WebSocket log")
}
