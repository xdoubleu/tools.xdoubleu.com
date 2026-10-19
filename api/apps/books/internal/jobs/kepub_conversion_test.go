package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/jobs"
	"tools.xdoubleu.com/internal/logging"
)

type fakeKEPUBDrainer struct {
	n   int
	err error
}

func (f *fakeKEPUBDrainer) Drain(_ context.Context) (int, error) { return f.n, f.err }

func TestKEPUBConversionJob(t *testing.T) {
	job := jobs.NewKEPUBConversionJob(&fakeKEPUBDrainer{n: 2, err: nil})

	assert.Equal(t, "books-kepub-conversion", job.ID())
	assert.Equal(t, time.Minute, job.RunEvery())
	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))
}

func TestKEPUBConversionJob_Error(t *testing.T) {
	want := errors.New("db down")
	job := jobs.NewKEPUBConversionJob(&fakeKEPUBDrainer{n: 0, err: want})

	require.ErrorIs(t, job.Run(t.Context(), logging.NewNopLogger()), want)
}
