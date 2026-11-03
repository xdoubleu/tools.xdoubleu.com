package factory_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/agentaccounts"
	"tools.xdoubleu.com/internal/models"
)

func accountsEnv(t *testing.T) jobEnv {
	t.Helper()
	return newJobEnvWith(t, true, "browser",
		factory.WithJobAccounts(agentaccounts.NewStore(testDB, nil)))
}

// accountOf is the account row of a job.
type accountOf struct {
	userID    string
	clientID  string
	name      string
	taskID    string
	active    bool
	rowsFound int
}

func jobAccount(t *testing.T, jobID string) accountOf {
	t.Helper()
	var a accountOf
	err := testDB.QueryRow(context.Background(), `
		SELECT user_id::text, client_id, name, task_id::text, revoked_at IS NULL
		FROM global.agent_accounts WHERE job_id = $1`, jobID,
	).Scan(&a.userID, &a.clientID, &a.name, &a.taskID, &a.active)
	if err == nil {
		a.rowsFound = 1
	}
	return a
}

func TestAClaimedJobGetsItsOwnAgentAccount(t *testing.T) {
	env := accountsEnv(t)
	task := env.newTask(t, "gets an identity")
	env.run(t, task.Id, "")

	claimed := env.claim(t)
	assert.NotEmpty(t, claimed.McpClientId)
	assert.NotEmpty(t, claimed.McpClientSecret)

	acc := jobAccount(t, claimed.Id)
	require.Equal(t, 1, acc.rowsFound)
	assert.Equal(t, claimed.McpClientId, acc.clientID)
	assert.Equal(t, task.Id, acc.taskID)
	assert.True(t, acc.active)
	assert.True(t, strings.HasPrefix(acc.name, "job #"+claimed.Id[:8]+" · "), acc.name)
}

func TestWithoutJobAccountsAClaimedJobHasNoIdentity(t *testing.T) {
	env := browserJobEnv(t)
	env.run(t, env.newTask(t, "no accounts").Id, "")
	claimed := env.claim(t)
	assert.Empty(t, claimed.McpClientId)
	assert.Empty(t, claimed.McpClientSecret)
}

func TestAJobsAccountEndsWithTheJob(t *testing.T) {
	env := accountsEnv(t)

	// Completed.
	done := env.newTask(t, "completes")
	env.run(t, done.Id, "")
	first := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: first.Id, Succeeded: true})
	require.NoError(t, err)
	assert.False(t, jobAccount(t, first.Id).active)
	assert.Equal(t, 1, jobAccount(t, first.Id).rowsFound, "the record stays")

	// Stopped.
	stopped := env.newTask(t, "stops")
	env.run(t, stopped.Id, "")
	second := env.claim(t)
	_, err = env.stop(t, stopped.Id)
	require.NoError(t, err)
	assert.False(t, jobAccount(t, second.Id).active)

	// Lost: noticed when the board is read.
	lost := env.newTask(t, "vanishes")
	env.run(t, lost.Id, "")
	third := env.claim(t)
	_, err = testDB.Exec(context.Background(),
		`UPDATE factory.jobs SET last_check_in = now() - interval '6 minutes'
		 WHERE id = $1`, third.Id)
	require.NoError(t, err)
	_, err = env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	assert.False(t, jobAccount(t, third.Id).active)

	// Each job had an account of its own.
	assert.NotEqual(t, jobAccount(t, first.Id).clientID, jobAccount(t, second.Id).clientID)
}

func TestAnAccountThatWasNotRevokedIsSweptOnTheNextRead(t *testing.T) {
	env := accountsEnv(t)
	task := env.newTask(t, "leaks")
	env.run(t, task.Id, "")
	job := env.claim(t)

	// The job ended without its account being revoked (a failed revoke).
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.jobs SET status = 'failed', finished_at = now()
		 WHERE id = $1`, job.Id)
	require.NoError(t, err)
	require.True(t, jobAccount(t, job.Id).active)

	env.task(t, task.Id) // any read sweeps
	assert.False(t, jobAccount(t, job.Id).active)

	// And one whose job row is gone altogether.
	other := env.newTask(t, "orphan")
	env.run(t, other.Id, "")
	orphan := env.claim(t)
	_, err = testDB.Exec(context.Background(),
		`DELETE FROM factory.jobs WHERE id = $1`, orphan.Id)
	require.NoError(t, err)
	env.task(t, task.Id)
	assert.False(t, jobAccount(t, orphan.Id).active)
}

type brokenAccounts struct{}

func (brokenAccounts) CreateForJob(
	context.Context, string, string, agentaccounts.Scope,
) (*agentaccounts.Account, string, error) {
	return nil, "", errors.New("accounts are down")
}

func (brokenAccounts) RevokeForJob(context.Context, uuid.UUID) error { return nil }

func (brokenAccounts) ActiveJobs(context.Context) ([]uuid.UUID, error) {
	return nil, nil
}

func (brokenAccounts) ScopeOf(
	context.Context, uuid.UUID,
) (*agentaccounts.Scope, error) {
	return nil, nil //nolint:nilnil // not scoped
}

var _ services.JobAccounts = brokenAccounts{}

func TestAJobThatCannotHaveAnAccountIsFailedNotRun(t *testing.T) {
	env := newJobEnvWith(t, true, "browser", factory.WithJobAccounts(brokenAccounts{}))
	task := env.newTask(t, "no identity")
	env.run(t, task.Id, "")

	_, err := env.runner.ClaimJob(bearer(env.token), &factoryv1.ClaimJobRequest{})
	require.Error(t, err)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	require.Len(t, got.Jobs, 1)
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_FAILED, got.Jobs[0].Status)
	require.Len(t, got.Comments, 1)
	assert.Contains(t, got.Comments[0].Body, "Could not create the job's agent account")
}

func TestAJobAccountSeesAndActsOnItsOwnTaskOnly(t *testing.T) {
	env := accountsEnv(t)
	mine := env.newTask(t, "my task")
	other := env.newTask(t, "someone else's task")
	env.run(t, mine.Id, "")
	claimed := env.claim(t)
	acc := jobAccount(t, claimed.Id)
	jobUser := &models.User{
		ID: acc.userID, Email: "job@agent.invalid", Role: models.RoleAgent,
		AppAccess: nil, HasMFA: false, DisplayName: "",
	}
	session := mcpSessionOn(t, env.app, jobUser)

	// The board holds both, but the job sees its own task only.
	list, err := callTool(t, session, "factory_list_tasks", map[string]any{})
	require.NoError(t, err)
	tasks, ok := list["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, tasks, 1)
	assert.Equal(t, mine.Id, str(t, tasks[0], "id"))

	got, err := callTool(t, session, "factory_get_task", map[string]any{"id": mine.Id})
	require.NoError(t, err)
	assert.Equal(t, "my task", str(t, got, "task", "title"))

	// Another task answers like one that doesn't exist.
	_, errOther := callTool(t, session, "factory_get_task", map[string]any{"id": other.Id})
	_, errNone := callTool(t, session, "factory_get_task",
		map[string]any{"id": uuid.NewString()})
	require.Error(t, errOther)
	require.Error(t, errNone)
	assert.Equal(t, errNone.Error(), errOther.Error())

	// Comments: its own task, and the comment names the job's account.
	_, err = callTool(t, session, "factory_comment_task",
		map[string]any{"task_id": mine.Id, "body": "Working on it."})
	require.NoError(t, err)
	comments := env.task(t, mine.Id).Comments
	require.Len(t, comments, 1)
	assert.Equal(t, "agent:"+acc.userID, comments[0].Author)
	_, err = callTool(t, session, "factory_comment_task",
		map[string]any{"task_id": other.Id, "body": "Nope."})
	require.Error(t, err)
	assert.Empty(t, env.task(t, other.Id).Comments)

	// It can't move tasks, its own included; the factory does that.
	_, err = callTool(t, session, "factory_move_task",
		map[string]any{"id": mine.Id, "status": "done"})
	require.Error(t, err)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS,
		env.task(t, mine.Id).Task.Status)

	// It can file new tasks into Backlog.
	made, err := callTool(t, session, "factory_create_task", map[string]any{
		"repo_id": seededRepoID(t), "title": "Follow-up", "body": "Found on the way.",
	})
	require.NoError(t, err)
	assert.Equal(t, "TASK_STATUS_BACKLOG", str(t, made, "task", "status"))
}

func TestAnAgentAccountMadeByAPersonStillSeesTheWholeBoard(t *testing.T) {
	env := accountsEnv(t)
	a := env.newTask(t, "first")
	b := env.newTask(t, "second")
	session := mcpSessionOn(t, env.app, asUser(models.RoleAgent))

	list, err := callTool(t, session, "factory_list_tasks", map[string]any{})
	require.NoError(t, err)
	tasks, ok := list["tasks"].([]any)
	require.True(t, ok)
	ids := []string{str(t, tasks[0], "id"), str(t, tasks[1], "id")}
	assert.ElementsMatch(t, []string{a.Id, b.Id}, ids)

	_, err = callTool(t, session, "factory_move_task",
		map[string]any{"id": a.Id, "status": "in_progress"})
	require.NoError(t, err)
}
