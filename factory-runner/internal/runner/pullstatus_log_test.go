package runner_test

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	require.NoError(t, err)
	defer f.Close()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	raw, err := io.ReadAll(gz)
	require.NoError(t, err)
	return string(raw)
}

// failedStatus reads the status of a pull request whose one red check has the
// given details URL, with logs answered as a redirect (as GitHub does) or refused.
func failedStatus(
	t *testing.T,
	detailsPath, log string,
	logStatus int,
) runner.PullStatus {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(openPull))
	})
	mux.HandleFunc("/repos/o/r/commits/abc/check-runs",
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `{"check_runs":[{"id":55,"name":"Test",`+
				`"status":"completed","conclusion":"failure","details_url":%q,`+
				`"output":{"title":"Test failed","summary":""}}]}`, srv.URL+detailsPath)
		})
	mux.HandleFunc("/repos/o/r/check-runs/55/annotations",
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	mux.HandleFunc("/repos/o/r/actions/jobs/55/logs",
		func(w http.ResponseWriter, r *http.Request) {
			if logStatus != http.StatusOK {
				w.WriteHeader(logStatus)
				return
			}
			http.Redirect(w, r, "/blob", http.StatusFound)
		})
	mux.HandleFunc("/blob", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(log))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := &runner.GitHubClient{BaseURL: srv.URL, HTTP: srv.Client()}
	st, err := c.PullRequestStatus(context.Background(), "tok", pullRefOf())
	require.NoError(t, err)
	require.Equal(t, runner.CIFailure, st.CI)
	require.Len(t, st.Failed, 1)
	return st
}

const jobPath = "/o/r/actions/runs/9/job/55"

func TestAFailedActionsJobBringsTheLogUpToItsFirstError(t *testing.T) {
	log := readFixture(t, "actions_job_failed.log.gz")
	st := failedStatus(t, jobPath, log, http.StatusOK)

	got := st.Failed[0].Excerpt
	assert.Contains(t, got, "Test failed")
	assert.Contains(t, got, "internal/runner/jobloop.go  78.9% of 19 changed lines")
	assert.Contains(t, got, "##[error]Process completed with exit code 1.")
	assert.NotContains(t, got, "Post job cleanup")
	assert.NotContains(t, got, "2026-11-14T")
	assert.NotContains(t, got, "\x1b")
	assert.LessOrEqual(t, len(got), 800+4096+2)
}

func TestALogWithoutAnErrorKeepsItsEnd(t *testing.T) {
	log := strings.Repeat("2026-01-02T03:04:05.0000000Z \x1b[31mnoise\x1b[0m\n", 600) +
		"2026-01-02T03:04:05.0000000Z last line\n"
	got := failedStatus(t, jobPath, log, http.StatusOK).Failed[0].Excerpt
	assert.True(t, strings.HasSuffix(got, "noise\nlast line"), got)
	assert.LessOrEqual(t, len(got), 800+4096+2)
}

func TestNoLogIsNotAFailure(t *testing.T) {
	for name, tc := range map[string]struct {
		path   string
		status int
	}{
		"app lacks actions:read": {jobPath, http.StatusForbidden},
		"not an Actions job":     {"/o/r/runs/9", http.StatusOK},
	} {
		t.Run(name, func(t *testing.T) {
			got := failedStatus(t, tc.path, "never read", tc.status).Failed[0].Excerpt
			assert.Equal(t, "Test failed", got)
		})
	}
}
