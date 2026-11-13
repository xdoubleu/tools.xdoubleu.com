package factory_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

func (e jobEnv) note(job, body string) error {
	_, err := e.runner.AddJobComment(bearer(e.token),
		&factoryv1.AddJobCommentRequest{JobId: job, Body: body})
	return err
}

func TestAProgressNoteLandsOnTheTaskWhileTheJobRuns(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)

	require.NoError(t, env.note(job.Id, "Attempt 2: CI failed on api-test, retrying"))
	require.NoError(t, env.note(job.Id, strings.Repeat("é", 5000)))

	resp, err := env.tasks.GetTask(context.Background(),
		&factoryv1.GetTaskRequest{Id: job.TaskId})
	require.NoError(t, err)
	var bodies []string
	for _, c := range resp.Comments {
		bodies = append(bodies, c.Body)
	}
	assert.Contains(t, bodies, "Attempt 2: CI failed on api-test, retrying")
	assert.Contains(t, bodies, strings.Repeat("é", 2000))
}

func TestAProgressNoteIsRefusedWhenEmptyUnknownOrTheJobHasEnded(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)

	assert.Equal(t, connect.CodeInvalidArgument, code(env.note(job.Id, "  ")))
	assert.Equal(t, connect.CodeInvalidArgument, code(env.note("nope", "x")))

	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: job.Id, Succeeded: true})
	require.NoError(t, err)
	assert.Equal(t, connect.CodeAlreadyExists, code(env.note(job.Id, "late")))
}
