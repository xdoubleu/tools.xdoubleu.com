package observability

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

// blockingJob runs until its context ends; swallow hides that, as SteamJob
// does with per-user errors.
type blockingJob struct {
	deadline time.Duration
	swallow  bool
}

func (b blockingJob) ID() string { return "blocking" }

func (b blockingJob) RunDeadline() time.Duration { return b.deadline }

func (b blockingJob) Run(ctx context.Context, _ *slog.Logger) error {
	<-ctx.Done()
	if b.swallow {
		return nil
	}
	return ctx.Err()
}

// deadlineProbeJob reports the deadline its run context carries.
type deadlineProbeJob struct {
	got *time.Time
}

func (d deadlineProbeJob) ID() string { return "probe" }

func (d deadlineProbeJob) Run(ctx context.Context, _ *slog.Logger) error {
	if dl, ok := ctx.Deadline(); ok {
		*d.got = dl
	}
	return nil
}

func TestTrackedJobRun_DeadlineCancelsAndLogsRun(t *testing.T) {
	for _, swallow := range []bool{false, true} {
		repo := &fakeInserter{runs: nil, insertErr: nil}
		job := newTestTrackedJob(
			blockingJob{deadline: 20 * time.Millisecond, swallow: swallow},
			repo,
		)
		var buf bytes.Buffer
		logger := slog.New(logging.NewBufLogHandler(&buf, nil))

		done := make(chan error, 1)
		go func() { done <- job.Run(t.Context(), logger) }()

		var err error
		select {
		case err = <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("run outlived its deadline")
		}

		require.ErrorIs(t, err, errRunDeadline)
		require.Len(t, repo.runs, 1)
		assert.False(t, repo.runs[0].Success)
		assert.Contains(t, repo.runs[0].Error, "run deadline")
		assert.Contains(t, buf.String(), "run deadline")
	}
}

func TestTrackedJobRun_DefaultDeadline(t *testing.T) {
	var got time.Time
	job := newTestTrackedJob(
		deadlineProbeJob{got: &got},
		&fakeInserter{runs: nil, insertErr: nil},
	)

	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))

	assert.WithinDuration(t, time.Now().Add(DefaultRunDeadline), got, time.Minute)
}

func TestTrackedJobRun_ParentCancelIsNotADeadline(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(blockingJob{deadline: time.Hour, swallow: false}, repo)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := job.Run(ctx, logging.NewNopLogger())
	require.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, errRunDeadline)
}
