package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

// capturingPoller records the context Run passes to Poll.
type capturingPoller struct {
	gotDeadline time.Time
	gotOK       bool
}

func (p *capturingPoller) Poll(ctx context.Context) error {
	p.gotDeadline, p.gotOK = ctx.Deadline()
	return nil
}

// TestRealtimePollJob_Run_BoundsPollWithATimeout: Run must hand Poll a
// deadline well under the 30s cadence.
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
