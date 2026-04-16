package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/cmd/publish/internal/logging"
)

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
