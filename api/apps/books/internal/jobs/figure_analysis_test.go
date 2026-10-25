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
	"tools.xdoubleu.com/internal/observability"
)

type fakeFigureAnalyser struct {
	limit int
	n     int
	err   error
}

func (f *fakeFigureAnalyser) AnalysePending(_ context.Context, limit int) (int, error) {
	f.limit = limit
	return f.n, f.err
}

func TestFigureAnalysisJob(t *testing.T) {
	analyser := &fakeFigureAnalyser{limit: 0, n: 3, err: nil}
	job := jobs.NewFigureAnalysisJob(analyser)

	assert.Equal(t, "books-figure-analysis", job.ID())
	assert.Equal(t, 15*time.Minute, job.RunEvery())
	require.NoError(t, job.Run(t.Context(), logging.NewNopLogger()))
	assert.Positive(t, analyser.limit)
}

func TestFigureAnalysisJob_RunDeadlineOutlastsDefault(t *testing.T) {
	var job observability.Deadlined = jobs.NewFigureAnalysisJob(nil)
	assert.Greater(t, job.RunDeadline(), observability.DefaultRunDeadline)
}

func TestFigureAnalysisJob_Error(t *testing.T) {
	want := errors.New("db down")
	job := jobs.NewFigureAnalysisJob(&fakeFigureAnalyser{limit: 0, n: 0, err: want})

	require.ErrorIs(t, job.Run(t.Context(), logging.NewNopLogger()), want)
}
