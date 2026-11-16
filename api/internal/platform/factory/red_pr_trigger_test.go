package factory_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func workflowRunEvent(t *testing.T, conclusion, branch, actorType, repo string) string {
	t.Helper()
	out, err := json.Marshal(map[string]any{
		"action": "completed",
		"workflow_run": map[string]any{
			"conclusion": conclusion, "head_branch": branch,
			"actor": map[string]any{"type": actorType},
		},
		"repository": map[string]any{
			"name": repo, "owner": map[string]any{"login": "xdoubleu"},
		},
	})
	require.NoError(t, err)
	return string(out)
}

func redPREnv(t *testing.T, enabled bool) jobEnv {
	t.Helper()
	env := webhookEnv(t)
	clean := func() {
		_, err := testDB.Exec(context.Background(), "DELETE FROM factory.jobs")
		require.NoError(t, err)
		_, err = testDB.Exec(context.Background(), "DELETE FROM factory.routines")
		require.NoError(t, err)
	}
	clean()
	t.Cleanup(clean)
	env.saveRoutine(t, "red-pr-repair", enabled, 0)
	return env
}

func queuedRoutineJobs(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT count(*) FROM factory.jobs WHERE routine = 'red-pr-repair'`).Scan(&n))
	return n
}

func deliverRun(t *testing.T, env jobEnv, body string) {
	t.Helper()
	assert.Equal(t, http.StatusNoContent,
		deliver(t, env, "workflow_run", body, sign(webhookSecret, body)))
}

func TestAFailedRunOnMainQueuesRedPRRepairOnce(t *testing.T) {
	env := redPREnv(t, true)

	deliverRun(
		t,
		env,
		workflowRunEvent(t, "failure", "main", "User", "tools.xdoubleu.com"),
	)
	deliverRun(
		t,
		env,
		workflowRunEvent(t, "failure", "main", "User", "tools.xdoubleu.com"),
	)

	assert.Equal(t, 1, queuedRoutineJobs(t))
}

func TestAFailedBotRunQueuesRedPRRepair(t *testing.T) {
	env := redPREnv(t, true)

	deliverRun(t, env,
		workflowRunEvent(t, "failure", "renovate/goose", "Bot", "tools.xdoubleu.com"))

	assert.Equal(t, 1, queuedRoutineJobs(t))
}

func TestOtherRunsQueueNoRedPRRepair(t *testing.T) {
	env := redPREnv(t, true)

	for _, body := range []string{
		workflowRunEvent(t, "success", "main", "User", "tools.xdoubleu.com"),
		workflowRunEvent(t, "cancelled", "main", "Bot", "tools.xdoubleu.com"),
		workflowRunEvent(t, "failure", "my-feature", "User", "tools.xdoubleu.com"),
		workflowRunEvent(t, "failure", "main", "Bot", "another-repo"),
		`{"action":"requested","workflow_run":{"conclusion":"failure","head_branch":"main"}}`,
	} {
		deliverRun(t, env, body)
	}

	assert.Equal(t, 0, queuedRoutineJobs(t))
}

func TestADisabledRedPRRepairIsNotQueuedByAFailedRun(t *testing.T) {
	env := redPREnv(t, false)

	deliverRun(t, env, workflowRunEvent(t, "failure", "main", "Bot", "tools.xdoubleu.com"))

	assert.Equal(t, 0, queuedRoutineJobs(t))
}

func TestAFailedRunWithoutTheRoutineIsIgnored(t *testing.T) {
	env := webhookEnv(t)
	_, err := testDB.Exec(context.Background(), "DELETE FROM factory.routines")
	require.NoError(t, err)

	deliverRun(t, env, workflowRunEvent(t, "failure", "main", "Bot", "tools.xdoubleu.com"))
}

func TestAMalformedWorkflowRunIsABadRequest(t *testing.T) {
	env := webhookEnv(t)
	assert.Equal(t, http.StatusBadRequest,
		deliver(t, env, "workflow_run", "{", sign(webhookSecret, "{")))
}
