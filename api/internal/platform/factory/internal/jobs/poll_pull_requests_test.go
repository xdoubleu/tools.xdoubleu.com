package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/factory/internal/jobs"
)

type fakePoller struct {
	n   int
	err error
}

func (f fakePoller) PollPullRequests(context.Context) (int, error) { return f.n, f.err }

func TestThePollJobRunsEveryFiveMinutesAndLogsWhatItRead(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	job := jobs.NewPollPullRequestsJob(fakePoller{n: 3, err: nil})
	assert.Equal(t, "factory-poll-pull-requests", job.ID())
	assert.Equal(t, 5*time.Minute, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), logger))
	assert.Contains(t, logs.String(), "count=3")

	logs.Reset()
	require.NoError(t, jobs.NewPollPullRequestsJob(fakePoller{n: 0, err: nil}).
		Run(context.Background(), logger))
	assert.Empty(t, logs.String(), "nothing to say when nothing was open")

	boom := errors.New("boom")
	require.ErrorIs(t, jobs.NewPollPullRequestsJob(fakePoller{n: 0, err: boom}).
		Run(context.Background(), logger), boom)
}
