package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

// capturingPoller records the context RealtimePollJob.Run hands to Poll, so
// the test can assert it carries a bounded deadline rather than the
// caller's own (possibly undeadlined) context.
type capturingPoller struct {
	gotDeadline time.Time
	gotOK       bool
}

func (p *capturingPoller) Poll(ctx context.Context) error {
	p.gotDeadline, p.gotOK = ctx.Deadline()
	return nil
}

// TestRealtimePollJob_Run_BoundsPollWithATimeout covers issue #1720: a slow
// step inside Poll — a slow SNCB response, or a trains.trips read blocked
// behind trains-static-import's TRUNCATE lock — must not be able to run
// past the next scheduled tick, so Run hands Poll a context with a deadline
// well under RunEvery's 30s cadence rather than the bare parent context.
func TestRealtimePollJob_Run_BoundsPollWithATimeout(t *testing.T) {
	poller := &capturingPoller{} //nolint:exhaustruct //fields populated by Poll
	job := NewRealtimePollJob(poller)

	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))

	require.True(t, poller.gotOK, "Poll's context should carry a deadline")
	remaining := time.Until(poller.gotDeadline)
	assert.Positive(t, remaining)
	assert.LessOrEqual(t, remaining, pollTimeout)
	assert.Less(
		t,
		pollTimeout,
		job.RunEvery(),
		"poll timeout must stay under the poll cadence",
	)
}
