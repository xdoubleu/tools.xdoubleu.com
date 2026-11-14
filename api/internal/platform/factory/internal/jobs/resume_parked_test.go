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
	n   int
	err error
}

func (f fakeResumer) ResumeParked(context.Context) (int, error) { return f.n, f.err }

func TestTheResumeJobRunsEveryFiveMinutesAndReportsFailure(t *testing.T) {
	job := jobs.NewResumeParkedJob(fakeResumer{n: 2, err: nil})
	assert.Equal(t, "factory-resume-parked", job.ID())
	assert.Equal(t, 5*time.Minute, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))

	boom := errors.New("boom")
	job = jobs.NewResumeParkedJob(fakeResumer{n: 0, err: boom})
	require.ErrorIs(t, job.Run(context.Background(), logging.NewNopLogger()), boom)
}
