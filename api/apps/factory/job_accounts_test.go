package factory_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/agentaccounts"
	"tools.xdoubleu.com/internal/models"
)

// fakeAccounts stands in for the agent account store: the factory only knows it
// through services.JobAccounts, and the store itself is tested against its own
// tables in internal/agentaccounts.
type fakeAccounts struct {
	mu       sync.Mutex
	accounts map[uuid.UUID]*accountOf
}

type accountOf struct {
	userID   string
	clientID string
	name     string
	taskID   uuid.UUID
	active   bool
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{mu: sync.Mutex{}, accounts: map[uuid.UUID]*accountOf{}}
}

func (f *fakeAccounts) CreateForJob(
	_ context.Context, _, name string, scope agentaccounts.Scope,
) (*agentaccounts.Account, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, taken := f.accounts[scope.JobID]; taken {
		return nil, "", errors.New("a job has at most one account")
	}
	acc := &accountOf{
		userID: uuid.NewString(), clientID: "agent-" + uuid.NewString(),
		name: name, taskID: scope.TaskID, active: true,
	}
	f.accounts[scope.JobID] = acc
	account := &agentaccounts.Account{
		ID: uuid.New(), Name: name, UserID: uuid.MustParse(acc.userID),
		ClientID: acc.clientID, CreatedBy: "", CreatedAt: time.Time{},
		RevokedAt: nil, JobID: &scope.JobID, TaskID: &scope.TaskID,
	}
	return account, "secret-" + acc.clientID, nil
}

func (f *fakeAccounts) RevokeForJob(_ context.Context, jobID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if acc, ok := f.accounts[jobID]; ok {
		acc.active = false
	}
	return nil
}

func (f *fakeAccounts) ActiveJobs(context.Context) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []uuid.UUID{}
	for id, acc := range f.accounts {
		if acc.active {
			out = append(out, id)
		}
	}
	return out, nil
}

// ScopeOf keeps the scope of a revoked account, like the store.
func (f *fakeAccounts) ScopeOf(
	_ context.Context, userID uuid.UUID,
) (*agentaccounts.Scope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for jobID, acc := range f.accounts {
		if acc.userID == userID.String() {
			return &agentaccounts.Scope{JobID: jobID, TaskID: acc.taskID}, nil
		}
	}
	return nil, nil //nolint:nilnil // not scoped
}

var _ services.JobAccounts = (*fakeAccounts)(nil)

// of is the account the job has, failing the test if it has none.
func (f *fakeAccounts) of(t *testing.T, jobID string) accountOf {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	acc, ok := f.accounts[uuid.MustParse(jobID)]
	require.True(t, ok, "job %s has no account", jobID)
	return *acc
}

func accountsEnv(t *testing.T) (jobEnv, *fakeAccounts) {
	t.Helper()
	accounts := newFakeAccounts()
	return newJobEnvWith(t, true, "browser", factory.WithJobAccounts(accounts)), accounts
}

func TestAClaimedJobGetsItsOwnAgentAccount(t *testing.T) {
	env, accounts := accountsEnv(t)
	task := env.newTask(t, "gets an identity")
	env.run(t, task.Id, "")

	claimed := env.claim(t)
	assert.NotEmpty(t, claimed.McpClientId)
	assert.NotEmpty(t, claimed.McpClientSecret)

	acc := accounts.of(t, claimed.Id)
	assert.Equal(t, claimed.McpClientId, acc.clientID)
	assert.Equal(t, task.Id, acc.taskID.String())
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
	env, accounts := accountsEnv(t)

	// Completed.
	done := env.newTask(t, "completes")
	env.run(t, done.Id, "")
	first := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: first.Id, Succeeded: true})
	require.NoError(t, err)
	assert.False(t, accounts.of(t, first.Id).active)

	// Stopped.
	stopped := env.newTask(t, "stops")
	env.run(t, stopped.Id, "")
	second := env.claim(t)
	_, err = env.stop(t, stopped.Id)
	require.NoError(t, err)
	assert.False(t, accounts.of(t, second.Id).active)

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
	assert.False(t, accounts.of(t, third.Id).active)

	// Each job had an account of its own.
	assert.NotEqual(t,
		accounts.of(t, first.Id).clientID, accounts.of(t, second.Id).clientID)
}

func TestAnAccountThatWasNotRevokedIsSweptOnTheNextRead(t *testing.T) {
	env, accounts := accountsEnv(t)
	task := env.newTask(t, "leaks")
	env.run(t, task.Id, "")
	job := env.claim(t)

	// The job ended without its account being revoked (a failed revoke).
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.jobs SET status = 'failed', finished_at = now()
		 WHERE id = $1`, job.Id)
	require.NoError(t, err)
	require.True(t, accounts.of(t, job.Id).active)

	env.task(t, task.Id) // any read sweeps
	assert.False(t, accounts.of(t, job.Id).active)

	// And one whose job row is gone altogether.
	other := env.newTask(t, "orphan")
	env.run(t, other.Id, "")
	orphan := env.claim(t)
	_, err = testDB.Exec(context.Background(),
		`DELETE FROM factory.jobs WHERE id = $1`, orphan.Id)
	require.NoError(t, err)
	env.task(t, task.Id)
	assert.False(t, accounts.of(t, orphan.Id).active)
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
	env, accounts := accountsEnv(t)
	mine := env.newTask(t, "my task")
	other := env.newTask(t, "someone else's task")
	env.run(t, mine.Id, "")
	claimed := env.claim(t)
	acc := accounts.of(t, claimed.Id)
	jobUser := &models.User{
		ID: acc.userID, Email: "job@agent.invalid", Role: models.RoleAgent,
		AppAccess: nil, WriteAccess: nil, HasMFA: false, DisplayName: "",
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
	env, _ := accountsEnv(t)
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

func TestAJobAccountStaysConfinedAfterItsJobEnds(t *testing.T) {
	env, accounts := accountsEnv(t)
	mine := env.newTask(t, "mine")
	other := env.newTask(t, "not mine")
	env.run(t, mine.Id, "")
	claimed := env.claim(t)
	acc := accounts.of(t, claimed.Id)
	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id, Succeeded: true})
	require.NoError(t, err)
	require.False(t, accounts.of(t, claimed.Id).active)

	// A request already past authentication when the job ended is not widened
	// into a person's agent account.
	session := mcpSessionOn(t, env.app, &models.User{
		ID: acc.userID, Email: "job@agent.invalid", Role: models.RoleAgent,
		AppAccess: nil, WriteAccess: nil, HasMFA: false, DisplayName: "",
	})
	_, err = callTool(t, session, "factory_get_task", map[string]any{"id": other.Id})
	require.Error(t, err)
	_, err = callTool(t, session, "factory_move_task",
		map[string]any{"id": other.Id, "status": "done"})
	require.Error(t, err)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG,
		env.task(t, other.Id).Task.Status)
}
