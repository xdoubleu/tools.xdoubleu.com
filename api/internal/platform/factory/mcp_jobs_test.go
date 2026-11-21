package factory_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/models"
)

func TestAgentReadsWhyAJobFailedAndItsLastEvents(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "blocked")
	env.run(t, task.Id)
	job := env.claim(t)

	lines := make([]string, 0, 60)
	for i := 1; i <= 60; i++ {
		lines = append(lines, fmt.Sprintf(`{"type":"assistant","n":%d}`, i))
	}
	// The runner redacts credential values before it sends a line.
	lines = append(lines, `{"type":"result","token":"[redacted]"}`)
	require.NoError(t, env.append(t, job.Id, 1, lines...))
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: job.Id, Succeeded: false, Outcome: "The agent exited with status 1.",
	})
	require.NoError(t, err)

	session := mcpSessionOn(t, env.app, asUser(models.RoleAgent))

	got, err := callTool(t, session, "factory_get_task", map[string]any{"id": task.Id})
	require.NoError(t, err)
	assert.Equal(t, job.Id, str(t, got, "jobs", 0, "id"))
	assert.Equal(t, "JOB_STATUS_FAILED", str(t, got, "jobs", 0, "status"))
	assert.Equal(t, "The agent exited with status 1.", str(t, got, "jobs", 0, "outcome"))

	res, err := callTool(t, session, "factory_get_job", map[string]any{"job_id": job.Id})
	require.NoError(t, err)
	assert.Equal(t, "The agent exited with status 1.", str(t, res, "job", "outcome"))
	events, ok := res["events"].([]any)
	require.True(t, ok)
	assert.Len(t, events, 50, "the default tail")
	assert.Equal(t, "result", str(t, events, 49, "kind"))
	assert.Contains(t, str(t, events, 49, "payload"), "[redacted]")
	assert.NotContains(t, str(t, events, 0, "payload"), `"n":1}`)

	res, err = callTool(t, session, "factory_get_job",
		map[string]any{"job_id": job.Id, "tail": 3})
	require.NoError(t, err)
	events, ok = res["events"].([]any)
	require.True(t, ok)
	assert.Len(t, events, 3)

	res, err = callTool(t, session, "factory_get_job",
		map[string]any{"job_id": job.Id, "tail": 5000})
	require.NoError(t, err)
	events, ok = res["events"].([]any)
	require.True(t, ok)
	assert.Len(t, events, 61, "capped at 200, here the whole log")

	// The prompt and session are not part of the answer.
	raw := fmt.Sprint(res)
	assert.False(t, strings.Contains(raw, "Do the thing."), "the prompt stays private")
}

func TestAStoppedJobHasAReasonAndGetJobRefusesBadIDs(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "stopped")
	started := env.run(t, task.Id)
	_, err := env.tasks.StopJob(t.Context(), &factoryv1.StopJobRequest{TaskId: task.Id})
	require.NoError(t, err)

	session := mcpSessionOn(t, env.app, asUser(models.RoleAgent))
	res, err := callTool(t, session, "factory_get_job",
		map[string]any{"job_id": started.Id})
	require.NoError(t, err)
	assert.Equal(t, "Stopped from the board.", str(t, res, "job", "outcome"))

	_, err = callTool(t, session, "factory_get_job", map[string]any{"job_id": "nope"})
	require.Error(t, err)
	_, err = callTool(t, session, "factory_get_job",
		map[string]any{"job_id": "00000000-0000-0000-0000-000000000000"})
	require.Error(t, err)
}
