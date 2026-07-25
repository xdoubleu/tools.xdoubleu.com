package observability

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/models"
)

type fakeInserter struct {
	runs      []models.JobRun
	insertErr error
}

func (f *fakeInserter) Insert(_ context.Context, run models.JobRun) error {
	f.runs = append(f.runs, run)
	return f.insertErr
}

type fakeJob struct {
	err    error
	panics bool
}

func (f fakeJob) ID() string { return "fake" }

func (f fakeJob) RunEvery() time.Duration { return time.Hour }

func (f fakeJob) Run(_ context.Context, _ *slog.Logger) error {
	if f.panics {
		panic("kaboom")
	}
	return f.err
}

func newTestTrackedJob(inner fakeJob, repo *fakeInserter) *TrackedJob {
	return &TrackedJob{inner: inner, repo: repo}
}

func TestTrackedJobDelegates(t *testing.T) {
	job := newTestTrackedJob(
		fakeJob{err: nil, panics: false},
		&fakeInserter{runs: nil, insertErr: nil},
	)

	assert.Equal(t, "fake", job.ID())
	assert.Equal(t, time.Hour, job.RunEvery())
}

func TestTrackedJobRecordsSuccess(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(fakeJob{err: nil, panics: false}, repo)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.NoError(t, err)

	require.Len(t, repo.runs, 1)
	assert.Equal(t, "fake", repo.runs[0].JobID)
	assert.True(t, repo.runs[0].Success)
	assert.Empty(t, repo.runs[0].Error)
	assert.WithinDuration(t, time.Now(), repo.runs[0].StartedAt, time.Second)
}

func TestTrackedJobRecordsFailure(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(fakeJob{err: errors.New("boom"), panics: false}, repo)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.EqualError(t, err, "boom")

	require.Len(t, repo.runs, 1)
	assert.False(t, repo.runs[0].Success)
	assert.Equal(t, "boom", repo.runs[0].Error)
}

func TestTrackedJobRecoversPanic(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: nil}
	job := newTestTrackedJob(fakeJob{err: nil, panics: true}, repo)

	err := job.Run(t.Context(), logging.NewNopLogger())
	require.ErrorContains(t, err, "kaboom")

	require.Len(t, repo.runs, 1)
	assert.False(t, repo.runs[0].Success)
	assert.Contains(t, repo.runs[0].Error, "kaboom")
}

func TestTrackedJobInsertFailureDoesNotFailJob(t *testing.T) {
	repo := &fakeInserter{runs: nil, insertErr: errors.New("db down")}
	job := newTestTrackedJob(fakeJob{err: nil, panics: false}, repo)

	err := job.Run(t.Context(), logging.NewNopLogger())
	assert.NoError(t, err)
}
