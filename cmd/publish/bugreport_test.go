package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/cmd/publish/internal/logging"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(
	r *http.Request,
) (*http.Response, error) {
	return f(r)
}

type bugReportFormData struct {
	Title       string `schema:"title"`
	Description string `schema:"description"`
}

func TestBugReportNotConfigured(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/api/bug-report",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(bugReportFormData{"Test bug", "Something broke"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusServiceUnavailable, rs.StatusCode)
}

func TestBugReportUnauthorized(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/api/bug-report",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(bugReportFormData{"Test bug", "Something broke"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnauthorized, rs.StatusCode)
}

func TestBugReportEmptyFields(t *testing.T) {
	tReq := test.CreateRequestTester(
		testAppWithGitHub.Routes(),
		http.MethodPost,
		"/api/bug-report",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(bugReportFormData{"", ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnprocessableEntity, rs.StatusCode)
}

func TestBugReportSuccess(t *testing.T) {
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

	tReq := test.CreateRequestTester(
		testAppWithGitHub.Routes(),
		http.MethodPost,
		"/api/bug-report",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(bugReportFormData{"Real bug", "Something broke on page /settings"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
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
	)

	assert.Contains(t, body, `failed \| with pipes and newline`)
	assert.Contains(t, body, "https://example.com/page")
	assert.Contains(t, body, "Mozilla/5.0")
	assert.Contains(t, body, "v1.2.3")
	assert.Contains(t, body, "production")
	assert.Contains(t, body, "user-123")
}
