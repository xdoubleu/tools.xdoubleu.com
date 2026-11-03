package factory_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/constants"
)

// append sends lines numbered first, first+1, and so on.
func (e jobEnv) append(
	t *testing.T,
	jobID string,
	first int32,
	lines ...string,
) error {
	t.Helper()
	numbered := make([]*factoryv1.JobLine, len(lines))
	for i, l := range lines {
		seq := first + int32(i)
		numbered[i] = &factoryv1.JobLine{Seq: seq, Line: l}
	}
	_, err := e.runner.AppendJobEvents(bearer(e.token),
		&factoryv1.AppendJobEventsRequest{JobId: jobID, Lines: numbered})
	return err
}

func (e jobEnv) events(t *testing.T, jobID string, after int32) []*factoryv1.JobEvent {
	t.Helper()
	resp, err := e.tasks.ListJobEvents(context.Background(),
		&factoryv1.ListJobEventsRequest{JobId: jobID, AfterSeq: after})
	require.NoError(t, err)
	return resp.Events
}

// runningJob queues and claims a job for a fresh task.
func (e jobEnv) runningJob(t *testing.T) *factoryv1.ClaimedJob {
	t.Helper()
	e.run(t, e.newTask(t, "logged").Id, "")
	return e.claim(t)
}

func TestARunningJobsLogIsStoredInOrderAndReadAfterASeq(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)

	require.NoError(t, env.append(t, job.Id, 1,
		`{"type":"system","session_id":"s"}`, `{"type":"assistant","text":"hi"}`))
	require.NoError(t, env.append(t, job.Id, 3, `not json`, `{"type":""}`))
	// A retried batch adds nothing.
	require.NoError(t, env.append(t, job.Id, 3, `not json`, `{"type":""}`))

	all := env.events(t, job.Id, 0)
	require.Len(t, all, 4)
	for i, e := range all {
		assert.EqualValues(t, i+1, e.Seq)
		assert.NotEmpty(t, e.CreatedAt)
	}
	assert.Equal(t, []string{"system", "assistant", "raw", "raw"},
		[]string{all[0].Kind, all[1].Kind, all[2].Kind, all[3].Kind})
	assert.Equal(t, `{"type":"assistant","text":"hi"}`, all[1].Payload)

	rest := env.events(t, job.Id, 2)
	require.Len(t, rest, 2)
	assert.EqualValues(t, 3, rest[0].Seq)
	assert.Empty(t, env.events(t, job.Id, 4))
	assert.Len(t, env.events(t, job.Id, -5), 4, "a negative cursor reads from the start")
}

func TestAJobLogTakesNothingOnceTheJobIsNotRunning(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)
	require.NoError(t, env.append(t, job.Id, 1, `{"type":"system"}`))

	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: job.Id, Succeeded: true})
	require.NoError(t, err)

	err = env.append(t, job.Id, 2, `{"type":"x"}`)
	assert.Equal(t, connect.CodeAlreadyExists, code(err))
	assert.Len(t, env.events(t, job.Id, 0), 1, "the log stays readable")
}

func TestAppendedEventsAreBoundedAndValidated(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)

	tooMany := make([]string, 201)
	err := env.append(t, job.Id, 1, tooMany...)
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	assert.Equal(t, connect.CodeInvalidArgument, code(env.append(t, "nope", 1, "x")))
	assert.Equal(t, connect.CodeNotFound, code(env.append(t, uuid.NewString(), 1, "x")))

	// A long line is cut on a rune boundary but keeps its type; a long type is
	// "raw"; a NUL is dropped; seqs outside 1..5000 are ignored.
	long := `{"type":"assistant","text":"` + strings.Repeat("é", 20000) + `"}`
	longKind := `{"type":"` + strings.Repeat("k", 41) + `"}`
	require.NoError(t, env.append(t, job.Id, 1, long, longKind, "a\x00b"))
	require.NoError(t, env.append(t, job.Id, 0, "zero"))
	require.NoError(t, env.append(t, job.Id, -3, "negative"))
	require.NoError(t, env.append(t, job.Id, 5001, "past the cap"))

	got := env.events(t, job.Id, 0)
	require.Len(t, got, 3)
	assert.LessOrEqual(t, len(got[0].Payload), 32*1024)
	assert.Equal(t, "assistant", got[0].Kind, "a cut line keeps its type")
	assert.Equal(t, "raw", got[1].Kind)
	assert.Equal(t, "ab", got[2].Payload)
}

func TestAJobLogStopsGrowingAtItsCap(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)

	batch := make([]string, 200)
	for i := range batch {
		batch[i] = `{"type":"assistant"}`
	}
	for i := range 25 {
		first := int32(i*200 + 1)
		require.NoError(t, env.append(t, job.Id, first, batch...))
	}
	require.NoError(t, env.append(t, job.Id, 5001, batch...))
	last := env.events(t, job.Id, 4999)
	require.Len(t, last, 1)
	assert.EqualValues(t, 5000, last[0].Seq)
}

func TestReadingAJobLogNeedsAHumanAdminInABrowser(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)

	for name, other := range map[string]factoryv1connect.TaskServiceClient{
		"not an admin":  newClientKeepingBoard(t, false, constants.ChannelBrowser),
		"not a browser": newClientKeepingBoard(t, true, constants.ChannelMCP),
	} {
		_, err := other.ListJobEvents(context.Background(),
			&factoryv1.ListJobEventsRequest{JobId: job.Id})
		assert.Equal(t, connect.CodePermissionDenied, code(err), name)
	}
	_, err := env.tasks.ListJobEvents(context.Background(),
		&factoryv1.ListJobEventsRequest{JobId: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = env.tasks.ListJobEvents(context.Background(),
		&factoryv1.ListJobEventsRequest{JobId: uuid.NewString()})
	assert.Equal(t, connect.CodeNotFound, code(err))
}

func TestTheRunnerCallsForLogsNeedADeviceToken(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)
	_, err := env.runner.AppendJobEvents(context.Background(),
		&factoryv1.AppendJobEventsRequest{
			JobId: job.Id, Lines: []*factoryv1.JobLine{{Seq: 1, Line: "x"}},
		})
	assert.Equal(t, connect.CodeUnauthenticated, code(err))
}

func TestOldJobLogsArePrunedAndRecentOnesKept(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)
	require.NoError(t, env.append(t, job.Id, 1, `{"type":"a"}`, `{"type":"b"}`))
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.job_events SET created_at = now() - interval '31 days'
		 WHERE job_id = $1 AND seq = 1`, job.Id)
	require.NoError(t, err)

	app := newApp(true, constants.ChannelBrowser)
	require.NoError(t, app.PruneEventsNow(context.Background()))

	left := env.events(t, job.Id, 0)
	require.Len(t, left, 1)
	assert.EqualValues(t, 2, left[0].Seq)
}
