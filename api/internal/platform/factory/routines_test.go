package factory_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
)

func routineEnv(t *testing.T) jobEnv {
	t.Helper()
	env := browserJobEnv(t)
	clean := func() {
		_, err := testDB.Exec(context.Background(), "DELETE FROM factory.jobs")
		require.NoError(t, err)
		_, err = testDB.Exec(context.Background(), "DELETE FROM factory.routines")
		require.NoError(t, err)
	}
	clean()
	t.Cleanup(clean)
	return env
}

func (e jobEnv) saveRoutine(
	t *testing.T,
	name string,
	enabled bool,
	interval int32,
) {
	t.Helper()
	_, err := e.tasks.SaveRoutine(context.Background(), &factoryv1.SaveRoutineRequest{
		Name: name, RepoId: seededRepoID(t), Enabled: enabled, IntervalMinutes: interval,
	})
	require.NoError(t, err)
}

func (e jobEnv) routines(t *testing.T) []*factoryv1.Routine {
	t.Helper()
	resp, err := e.tasks.ListRoutines(context.Background(),
		&factoryv1.ListRoutinesRequest{})
	require.NoError(t, err)
	return resp.Routines
}

func TestAManuallyRunRoutineRunsToTheEndWithoutATask(t *testing.T) {
	env := routineEnv(t)
	env.saveRoutine(t, "noop", false, 0)

	resp, err := env.tasks.RunRoutine(context.Background(),
		&factoryv1.RunRoutineRequest{Name: "noop"})
	require.NoError(t, err)
	assert.Equal(t, "noop", resp.Job.Routine)
	assert.Empty(t, resp.Job.TaskId)

	_, err = env.tasks.RunRoutine(context.Background(),
		&factoryv1.RunRoutineRequest{Name: "noop"})
	assert.Equal(t, connect.CodeAlreadyExists, code(err), "one unfinished job at a time")

	claimed := env.claim(t)
	require.NotNil(t, claimed)
	assert.Equal(t, "noop", claimed.Routine)
	assert.Empty(t, claimed.TaskId)
	assert.Empty(t, claimed.McpClientId)
	assert.Equal(t, "agent/routine-noop", claimed.Branch)

	stop, err := env.runner.CheckIn(bearer(env.token),
		&factoryv1.CheckInRequest{JobId: claimed.Id})
	require.NoError(t, err)
	assert.False(t, stop.Stop)
	_, err = env.runner.AddJobComment(bearer(env.token),
		&factoryv1.AddJobCommentRequest{JobId: claimed.Id, Body: "working"})
	require.NoError(t, err)

	_, err = env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, Succeeded: true, Outcome: "No-op ran.",
	})
	require.NoError(t, err)

	list := env.routines(t)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].LastJob)
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_SUCCEEDED, list[0].LastJob.Status)
}

func TestStopRoutineCancelsItsJob(t *testing.T) {
	env := routineEnv(t)
	env.saveRoutine(t, "noop", false, 0)
	_, err := env.tasks.StopRoutine(context.Background(),
		&factoryv1.StopRoutineRequest{Name: "noop"})
	assert.Equal(t, connect.CodeAlreadyExists, code(err), "nothing to stop")

	_, err = env.tasks.RunRoutine(context.Background(),
		&factoryv1.RunRoutineRequest{Name: "noop"})
	require.NoError(t, err)
	claimed := env.claim(t)
	require.NotNil(t, claimed)

	stopped, err := env.tasks.StopRoutine(context.Background(),
		&factoryv1.StopRoutineRequest{Name: "noop"})
	require.NoError(t, err)
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_CANCELLED, stopped.Job.Status)

	stop, err := env.runner.CheckIn(bearer(env.token),
		&factoryv1.CheckInRequest{JobId: claimed.Id})
	require.NoError(t, err)
	assert.True(t, stop.Stop)
}

func TestOnlyAHumanAdminInABrowserChangesOrRunsARoutine(t *testing.T) {
	env := routineEnv(t)
	env.saveRoutine(t, "noop", false, 0)

	for name, other := range map[string]factoryv1connect.TaskServiceClient{
		"not an admin":  newClientKeepingBoard(t, false, constants.ChannelBrowser),
		"not a browser": newClientKeepingBoard(t, true, constants.ChannelMCP),
	} {
		_, err := other.SaveRoutine(context.Background(), &factoryv1.SaveRoutineRequest{
			Name: "noop", RepoId: seededRepoID(t), Enabled: true, IntervalMinutes: 5,
		})
		assert.Equal(t, connect.CodePermissionDenied, code(err), name)
		_, err = other.RunRoutine(context.Background(),
			&factoryv1.RunRoutineRequest{Name: "noop"})
		assert.Equal(t, connect.CodePermissionDenied, code(err), name)
		_, err = other.StopRoutine(context.Background(),
			&factoryv1.StopRoutineRequest{Name: "noop"})
		assert.Equal(t, connect.CodePermissionDenied, code(err), name)
	}
}

func TestSaveRoutineRefusesABadNameOrInterval(t *testing.T) {
	env := routineEnv(t)
	for _, in := range []*factoryv1.SaveRoutineRequest{
		{Name: "Bad Name", RepoId: seededRepoID(t)},
		{Name: "ok", RepoId: seededRepoID(t), IntervalMinutes: -1},
	} {
		_, err := env.tasks.SaveRoutine(context.Background(), in)
		assert.Equal(t, connect.CodeInvalidArgument, code(err))
	}
}

func TestTheScheduleQueuesADueRoutineOnceAndSkipsTheRest(t *testing.T) {
	env := routineEnv(t)
	env.saveRoutine(t, "due", true, 60)
	env.saveRoutine(t, "off", false, 60)
	env.saveRoutine(t, "manual", true, 0)

	require.NoError(t, env.app.EnqueueRoutinesNow(context.Background()))
	require.NoError(t, env.app.EnqueueRoutinesNow(context.Background()))

	var queued int
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT count(*) FROM factory.jobs WHERE status = 'queued'`).Scan(&queued))
	assert.Equal(t, 1, queued)
	claimed := env.claim(t)
	require.NotNil(t, claimed)
	assert.Equal(t, "due", claimed.Routine)

	// Finished within its interval: not due again yet.
	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id, Succeeded: true})
	require.NoError(t, err)
	require.NoError(t, env.app.EnqueueRoutinesNow(context.Background()))
	assert.Nil(t, env.claim(t))
}

func TestThePausedBoardQueuesNoRoutine(t *testing.T) {
	env := routineEnv(t)
	env.saveRoutine(t, "due", true, 60)
	_, err := env.tasks.SetPickupPaused(context.Background(),
		&factoryv1.SetPickupPausedRequest{Paused: true})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.tasks.SetPickupPaused(context.Background(),
			&factoryv1.SetPickupPausedRequest{Paused: false})
	})

	require.NoError(t, env.app.EnqueueRoutinesNow(context.Background()))
	assert.Nil(t, env.claim(t))
}
