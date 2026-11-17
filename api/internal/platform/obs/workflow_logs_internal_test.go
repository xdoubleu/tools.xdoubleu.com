package obs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/github"
)

type logsGithub struct {
	github.Client

	runs    []github.WorkflowRun
	jobs    map[int64][]github.WorkflowJob
	logs    map[int64]string
	logsErr error
	runsErr error
}

func (f logsGithub) ListWorkflowRuns(context.Context) ([]github.WorkflowRun, error) {
	return f.runs, f.runsErr
}

func (f logsGithub) ListWorkflowRunJobs(
	_ context.Context, id int64,
) ([]github.WorkflowJob, error) {
	return f.jobs[id], nil
}

func (f logsGithub) GetJobLogs(_ context.Context, id int64) (string, error) {
	return f.logs[id], f.logsErr
}

func wfRun(
	id int64, event, branch string, pr int64, conclusion string,
) github.WorkflowRun {
	return github.WorkflowRun{
		ID: id, Name: "", Event: event, Branch: branch, Status: "completed",
		Conclusion: conclusion, URL: "", StartedAt: time.Time{}, DurationMs: 0,
		PRNumber: pr,
	}
}

func wfJob(id int64, name, conclusion string) github.WorkflowJob {
	return github.WorkflowJob{
		ID: id, Name: name, Status: "completed", Conclusion: conclusion,
		StartedAt: time.Time{}, CompletedAt: time.Time{}, DurationMs: 0,
	}
}

func newLogsGithub() logsGithub {
	//nolint:exhaustruct // the embedded interface and error fields stay unset
	return logsGithub{
		runs: []github.WorkflowRun{
			{ID: 1, Event: "pull_request", PRNumber: 7, Conclusion: "failure"},
			{ID: 2, Event: "pull_request", PRNumber: 8, Conclusion: "failure"},
			{ID: 3, Event: "pull_request", PRNumber: 7, Conclusion: "success"},
			{ID: 4, Event: "pull_request", PRNumber: 7, Conclusion: "failure"},
			{ID: 5, Event: "push", Branch: "main", Conclusion: "failure"},
		},
		jobs: map[int64][]github.WorkflowJob{
			1: {
				{ID: 10, Name: "api-lint", Conclusion: "failure"},
				{ID: 11, Name: "api-test", Conclusion: "success"},
				{ID: 12, Name: "web-test", Conclusion: "timed_out"},
			},
			4: {{ID: 40, Name: "build", Conclusion: "failure"}},
		},
		logs: map[int64]string{
			10: "\x1b[31mred\x1b[0m error\n",
			12: "slow",
			40: "boom",
		},
	}
}

func logsHandler(gh github.Client) *obsConnectHandler {
	return signalHandler(nil, gh, nil)
}

func TestWorkflowRunLogs_ByPRNumberCoversFailedRunsOnly(t *testing.T) {
	resp, err := logsHandler(newLogsGithub()).
		workflowRunLogs(context.Background(), 0, 7, "")
	require.NoError(t, err)
	require.Len(t, resp.Jobs, 3)
	assert.True(t, resp.Configured)
	assert.Equal(t, "red error", resp.Jobs[0].Log, "ANSI is stripped")
	assert.Equal(t, int64(1), resp.Jobs[0].RunId)
	assert.Equal(t, "web-test", resp.Jobs[1].Job)
	assert.Equal(t, "timed_out", resp.Jobs[1].Conclusion)
	assert.Equal(t, int64(4), resp.Jobs[2].RunId)
}

func TestWorkflowRunLogs_ByRunIDWithJobFilter(t *testing.T) {
	resp, err := logsHandler(newLogsGithub()).
		workflowRunLogs(context.Background(), 1, 0, "LINT")
	require.NoError(t, err)
	require.Len(t, resp.Jobs, 1)
	assert.Equal(t, "api-lint", resp.Jobs[0].Job)
}

func TestWorkflowRunLogs_RequiresRunOrPR(t *testing.T) {
	_, err := logsHandler(newLogsGithub()).
		workflowRunLogs(context.Background(), 0, 0, "")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestWorkflowRunLogs_NotConfigured(t *testing.T) {
	gh := newLogsGithub()
	gh.runsErr = github.ErrNotConfigured
	resp, err := logsHandler(gh).workflowRunLogs(context.Background(), 0, 7, "")
	require.NoError(t, err)
	assert.False(t, resp.Configured)
}

func TestWorkflowRunLogs_FetchErrorIsReported(t *testing.T) {
	gh := newLogsGithub()
	gh.logsErr = errors.New("gone")
	resp, err := logsHandler(gh).workflowRunLogs(context.Background(), 1, 0, "")
	require.NoError(t, err)
	assert.True(t, resp.Configured)
	assert.Contains(t, resp.Error, "gone")
}

func TestWorkflowRunLogs_CapsJobCount(t *testing.T) {
	gh := newLogsGithub()
	var jobs []github.WorkflowJob
	for i := range 9 {
		jobs = append(jobs, wfJob(int64(100+i), "j", "failure"))
	}
	gh.jobs[1] = jobs
	resp, err := logsHandler(gh).workflowRunLogs(context.Background(), 1, 0, "")
	require.NoError(t, err)
	assert.Len(t, resp.Jobs, maxLogJobs)
}

func TestTailLog(t *testing.T) {
	short, truncated := tailLog("  a\x1b[1mb\n", 100)
	assert.Equal(t, "ab", short)
	assert.False(t, truncated)

	text := strings.Repeat("x", 50) + "\nlast line\n"
	tail, truncated := tailLog(text, 15)
	assert.True(t, truncated)
	assert.Equal(t, "last line", tail, "starts at a line boundary")

	tail, _ = tailLog(strings.Repeat("é", 20), 5)
	assert.Equal(t, "éé", tail, "never splits a rune")
}

func TestFailedJobNames_PRRunAndMain(t *testing.T) {
	h := logsHandler(newLogsGithub())
	ctx := context.Background()
	pr := wfRun(1, "pull_request", "", 0, "failure")
	assert.Equal(t, []string{"api-lint", "web-test"}, h.failedJobNames(ctx, pr))

	pr.Conclusion = "success"
	assert.Empty(t, h.failedJobNames(ctx, pr))

	other := wfRun(1, "push", "x", 0, "failure")
	assert.Empty(t, h.failedJobNames(ctx, other))
}
