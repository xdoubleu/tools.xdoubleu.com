package github_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const failingCheckRun = `{"check_runs":[{"name":"ci-pass","status":"completed",` +
	`"conclusion":"failure","html_url":"u"}]}`

func TestListFailingPullRequests_IncludesClaudeBranchPRs(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/" + testRepo + "/pulls":
				_, _ = w.Write([]byte(`[
					{"number":1,"title":"Agent fix","html_url":"u",
					 "updated_at":"2026-07-01T10:00:00Z","user":{"login":"bot"},
					 "head":{"sha":"sha1","ref":"claude/fix-thing"}},
					{"number":2,"title":"Human work","html_url":"u",
					 "updated_at":"2026-07-01T10:00:00Z","user":{"login":"bob"},
					 "head":{"sha":"sha2","ref":"feature/claude-ish"}}
				]`))
			case "/repos/" + testRepo + "/commits/sha1/check-runs":
				_, _ = w.Write([]byte(failingCheckRun))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	defer cleanup()

	prs, err := newClient().ListFailingPullRequests(context.Background())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, int64(1), prs[0].Number)
	assert.Equal(t, "claude/fix-thing", prs[0].HeadRef)
}

func TestListFailingPullRequests_PagesThroughPullsAndCheckRuns(t *testing.T) {
	outOfScope := make([]string, 100)
	for i := range outOfScope {
		outOfScope[i] = fmt.Sprintf(`{"number":%d,"head":{"sha":"x","ref":"main"}}`, i+10)
	}
	passing := make([]string, 100)
	for i := range passing {
		passing[i] = fmt.Sprintf(
			`{"name":"job-%d","status":"completed","conclusion":"success"}`, i,
		)
	}

	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.Equal(t, "100", r.URL.Query().Get("per_page"))
			page := r.URL.Query().Get("page")
			switch r.URL.Path {
			case "/repos/" + testRepo + "/pulls":
				if page == "1" {
					_, _ = w.Write([]byte("[" + strings.Join(outOfScope, ",") + "]"))
					return
				}
				_, _ = w.Write([]byte(`[{"number":1,"title":"Bump","html_url":"u",
					"updated_at":"2026-07-01T10:00:00Z","user":{"login":"renovate"},
					"head":{"sha":"sha1","ref":"renovate/foo"},
					"labels":[{"name":"dependencies"}]}]`))
			case "/repos/" + testRepo + "/commits/sha1/check-runs":
				if page == "1" {
					_, _ = w.Write([]byte(`{"check_runs":[` +
						strings.Join(passing, ",") + `]}`))
					return
				}
				_, _ = w.Write([]byte(failingCheckRun))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	defer cleanup()

	prs, err := newClient().ListFailingPullRequests(context.Background())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, int64(1), prs[0].Number)
	require.Len(t, prs[0].FailingChecks, 1)
	assert.Equal(t, "ci-pass", prs[0].FailingChecks[0].Name)
}
