package runner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func pullRefOf() runner.PullRef {
	return runner.PullRef{Owner: "o", Repo: "r", Number: 7}
}

// githubServing answers GitHub's pull request, check-runs and annotations
// routes from the given bodies; a "" body is a 500.
func githubServing(
	t *testing.T,
	pull, runs, annotations string,
) *runner.GitHubClient {
	t.Helper()
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		body := ""
		switch {
		case r.URL.Path == "/repos/o/r/pulls/7":
			body = pull
		case strings.HasSuffix(r.URL.Path, "/check-runs"):
			body = runs
		case strings.HasSuffix(r.URL.Path, "/annotations"):
			body = annotations
		}
		if body == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(body))
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	return &runner.GitHubClient{BaseURL: srv.URL, HTTP: srv.Client()}
}

const openPull = `{"state":"open","merged":false,"head":{"sha":"abc"}}`

func TestAMergedOrClosedPullRequestNeedsNoCheckRuns(t *testing.T) {
	c := githubServing(t, `{"state":"closed","merged":true,"head":{"sha":"abc"}}`, "", "")
	st, err := c.PullRequestStatus(context.Background(), "tok", pullRefOf())
	require.NoError(t, err)
	assert.Equal(t, runner.PullMerged, st.State)

	c = githubServing(t, `{"state":"closed","merged":false,"head":{"sha":"abc"}}`, "", "")
	st, err = c.PullRequestStatus(context.Background(), "tok", pullRefOf())
	require.NoError(t, err)
	assert.Equal(t, runner.PullClosed, st.State)
}

func TestCIIsPendingUntilCiPassHasFinished(t *testing.T) {
	for name, runs := range map[string]string{
		"no checks yet": `{"check_runs":[]}`,
		"ci-pass queued": `{"check_runs":[{"id":1,"name":"ci-pass","status":"queued",` +
			`"conclusion":null}]}`,
		"others red, ci-pass not done": `{"check_runs":[{"id":2,"name":"api-test",` +
			`"status":"completed","conclusion":"failure"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			c := githubServing(t, openPull, runs, "[]")
			st, err := c.PullRequestStatus(context.Background(), "tok", pullRefOf())
			require.NoError(t, err)
			assert.Equal(t, runner.PullOpen, st.State)
			assert.Equal(t, "abc", st.HeadSHA)
			assert.Equal(t, runner.CIPending, st.CI)
			assert.Empty(t, st.Failed)
		})
	}
}

func TestACompletedGreenCiPassIsSuccess(t *testing.T) {
	c := githubServing(t, openPull, `{"check_runs":[`+
		`{"id":1,"name":"ci-pass","status":"completed","conclusion":"success"},`+
		`{"id":2,"name":"optional","status":"completed","conclusion":"failure"}]}`, "[]")
	st, err := c.PullRequestStatus(context.Background(), "tok", pullRefOf())
	require.NoError(t, err)
	assert.Equal(t, runner.CISuccess, st.CI)
}

func TestARedCiPassNamesTheFailedChecksWithTheirAnnotations(t *testing.T) {
	c := githubServing(t, openPull, `{"check_runs":[`+
		`{"id":1,"name":"ci-pass","status":"completed","conclusion":"failure"},`+
		`{"id":2,"name":"api-test","status":"completed","conclusion":"failure",`+
		`"output":{"title":"3 failed","summary":"see below"}},`+
		`{"id":3,"name":"web-lint","status":"completed","conclusion":"timed_out"},`+
		`{"id":4,"name":"docs","status":"completed","conclusion":"success"}]}`,
		`[{"path":"a.go","start_line":12,"annotation_level":"failure","message":"boom"},`+
			`{"path":"b.go","start_line":1,"annotation_level":"notice","message":"fyi"}]`)

	st, err := c.PullRequestStatus(context.Background(), "tok", pullRefOf())
	require.NoError(t, err)

	assert.Equal(t, runner.CIFailure, st.CI)
	require.Len(t, st.Failed, 2)
	assert.Equal(t, "api-test", st.Failed[0].Name)
	assert.Contains(t, st.Failed[0].Excerpt, "3 failed see below")
	assert.Contains(t, st.Failed[0].Excerpt, "a.go:12: boom")
	assert.NotContains(t, st.Failed[0].Excerpt, "fyi")
	assert.Equal(t, "web-lint", st.Failed[1].Name)
}

func TestARedCiPassAloneIsNamedAndALongExcerptIsCut(t *testing.T) {
	long := strings.Repeat("x", 3000)
	c := githubServing(t, openPull, `{"check_runs":[`+
		`{"id":1,"name":"ci-pass","status":"completed","conclusion":"failure",`+
		`"output":{"title":"`+long+`","summary":""}}]}`, "")

	st, err := c.PullRequestStatus(context.Background(), "tok", pullRefOf())
	require.NoError(t, err)

	require.Len(t, st.Failed, 1)
	assert.Equal(t, "ci-pass", st.Failed[0].Name)
	assert.LessOrEqual(t, len(st.Failed[0].Excerpt), 810, "cut")
}

func TestAnUnreadablePullRequestOrCheckListIsAnError(t *testing.T) {
	c := githubServing(t, "", "", "")
	_, err := c.PullRequestStatus(context.Background(), "tok", pullRefOf())
	require.ErrorContains(t, err, "read the pull request")

	c = githubServing(t, openPull, "", "")
	_, err = c.PullRequestStatus(context.Background(), "tok", pullRefOf())
	require.ErrorContains(t, err, "read the checks")
}

func TestAddJobCommentPostsTheNote(t *testing.T) {
	var body string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/factory.v1.RunnerService/AddJobComment", r.URL.Path)
		raw := make([]byte, 300)
		n, _ := r.Body.Read(raw)
		body = string(raw[:n])
		_, _ = w.Write([]byte(`{}`))
	})
	require.NoError(t, c.AddJobComment(context.Background(), "frt_x", "j1", "Attempt 2"))
	assert.JSONEq(t, `{"jobId":"j1","body":"Attempt 2"}`, body)
}
