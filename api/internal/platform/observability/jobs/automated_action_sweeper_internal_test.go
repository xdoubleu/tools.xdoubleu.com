package jobs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAutomatedActionCloser struct {
	ids    []int64
	err    error
	cutoff time.Time
}

func (s *stubAutomatedActionCloser) CloseStale(
	_ context.Context, cutoff time.Time,
) ([]int64, error) {
	s.cutoff = cutoff
	return s.ids, s.err
}

func TestAutomatedActionSweepJobClosesStaleRows(t *testing.T) {
	stub := &stubAutomatedActionCloser{
		ids: []int64{5, 21}, err: nil, cutoff: time.Time{},
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	job := NewAutomatedActionSweepJob(stub)
	require.NoError(t, job.Run(t.Context(), logger))

	assert.Equal(t, []int64{5, 21}, stub.ids)
	assert.WithinDuration(
		t, time.Now().Add(-staleActionMaxAge), stub.cutoff, time.Minute,
	)
	assert.Contains(t, buf.String(), "closed stale automated_actions rows")
}

func TestAutomatedActionSweepJobNoStaleRows(t *testing.T) {
	stub := &stubAutomatedActionCloser{ids: nil, err: nil, cutoff: time.Time{}}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	require.NoError(t, NewAutomatedActionSweepJob(stub).Run(t.Context(), logger))

	assert.Empty(t, buf.String())
}

func TestAutomatedActionSweepJobCloseErrorIsSwallowed(t *testing.T) {
	// A sweep failure must not fail the run; the next sweep retries.
	stub := &stubAutomatedActionCloser{
		ids: nil, err: errors.New("db down"), cutoff: time.Time{},
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	require.NoError(t, NewAutomatedActionSweepJob(stub).Run(t.Context(), logger))

	assert.Contains(t, buf.String(), "failed to close stale rows")
}
