package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/platform/factory/internal/jobs"
)

type fakeResumer struct {
	n       int
	err     error
	waited  int
	waitErr error
}

func (f fakeResumer) ResumeParked(context.Context) (int, error) { return f.n, f.err }

func (f fakeResumer) ResumeWaiting(context.Context) (int, error) {
	return f.waited, f.waitErr
}

func TestTheResumeJobRunsEveryFiveMinutesAndReportsFailure(t *testing.T) {
	job := jobs.NewResumeParkedJob(fakeResumer{n: 2, err: nil, waited: 1, waitErr: nil})
	assert.Equal(t, "factory-resume-parked", job.ID())
	assert.Equal(t, 5*time.Minute, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))

	boom := errors.New("boom")
	job = jobs.NewResumeParkedJob(fakeResumer{n: 0, err: boom, waited: 0, waitErr: nil})
	require.ErrorIs(t, job.Run(context.Background(), logging.NewNopLogger()), boom)
}

func TestTheResumeJobStillResumesWaitingTasksWhenParkedFails(t *testing.T) {
	parked, waiting := errors.New("parked"), errors.New("waiting")
	job := jobs.NewResumeParkedJob(
		fakeResumer{n: 0, err: parked, waited: 0, waitErr: waiting})
	err := job.Run(context.Background(), logging.NewNopLogger())
	require.ErrorIs(t, err, parked)
	require.ErrorIs(t, err, waiting)
}
