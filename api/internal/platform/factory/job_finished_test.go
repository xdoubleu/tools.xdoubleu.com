package factory_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

// finishedJob runs a task's job to the end, with the harness session "sess-1",
// and returns the job as claimed.
func (e jobEnv) finishedJob(t *testing.T, taskID string) *factoryv1.ClaimedJob {
	t.Helper()
	e.run(t, taskID)
	claimed := e.claim(t)
	require.NotNil(t, claimed)
	_, err := e.runner.CheckIn(bearer(e.token),
		&factoryv1.CheckInRequest{JobId: claimed.Id, SessionId: "sess-1"})
	require.NoError(t, err)
	_, err = e.runner.CompleteJob(bearer(e.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id, Succeeded: true})
	require.NoError(t, err)
	return claimed
}
