package factory_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

type seeded struct {
	id     string
	number int64
}

func (s seeded) ref() string { return fmt.Sprintf("T-%d", s.number) }

// seedTask inserts a task directly with its body as the approved text.
func seedTask(
	t *testing.T,
	title, body, status string,
	labels []string,
) seeded {
	t.Helper()
	var s seeded
	if labels == nil {
		labels = []string{}
	}
	require.NoError(t, testDB.QueryRow(context.Background(), `
		INSERT INTO factory.tasks (title, body, status, labels, repo_id, created_by,
			body_edited_by, body_edited_channel, ready_title, ready_body)
		VALUES ($1, $2, $3, $4, $5, 'human:x', 'human:x', 'browser', $1, $2)
		RETURNING id, number`,
		title, body, status, labels, seededRepoID(t)).Scan(&s.id, &s.number))
	t.Cleanup(func() {
		_, err := testDB.Exec(context.Background(),
			`DELETE FROM factory.tasks WHERE id = $1`, s.id)
		require.NoError(t, err)
	})
	return s
}

func (e jobEnv) assertSkipped(t *testing.T, s seeded, reason string) {
	t.Helper()
	got := e.task(t, s.id).Task
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_READY, got.Status)
	assert.Contains(t, got.PickupBlockReason, reason)
}

func TestPickupSkipsAnEpic(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, true)
	epic := seedTask(t, "epic", "Do it.", "ready", []string{"epic"})

	assert.Nil(t, env.claim(t))
	env.assertSkipped(t, epic, "epic")
}

func TestPickupSkipsTheParentOfAnotherTask(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, true)
	parent := seedTask(t, "parent", "Do it.", "ready", nil)
	seedTask(t, "child", "Part of parent ("+parent.ref()+")\n\nDo it.", "backlog", nil)

	assert.Nil(t, env.claim(t))
	env.assertSkipped(t, parent, "parent of")
}

func TestPickupSkipsATaskWhoseTaskDependencyIsNotMerged(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, true)
	dep := seedTask(t, "dep", "Do it.", "in_review", nil)
	task := seedTask(t, "task", "Depends on: "+dep.ref()+"\n\nDo it.", "ready", nil)

	assert.Nil(t, env.claim(t))
	env.assertSkipped(t, task, "depends on "+dep.ref())

	// Once the dependency is done the skip lifts by itself.
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.tasks SET status = 'done' WHERE id = $1`, dep.id)
	require.NoError(t, err)
	claimed := env.claim(t)
	require.NotNil(t, claimed)
	assert.Equal(t, task.ref(), claimed.TaskId)
	assert.Empty(t, env.task(t, task.id).Task.PickupBlockReason)
}

func TestPickupSkipsATaskWhoseIssueDependencyIsNotMerged(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, true)
	seedTask(t, "imported", "[github-issue:900001]\n\nDo it.", "in_progress", nil)
	task := seedTask(t, "task", "Depends on: #900001\n\nDo it.", "ready", nil)

	assert.Nil(t, env.claim(t))
	env.assertSkipped(t, task, "GitHub issue #900001")
}

func TestPickupIgnoresADependencyOnAnIssueNoTaskHolds(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, true)
	task := seedTask(t, "task", "Depends on: #900002\n\nDo it.", "ready", nil)

	claimed := env.claim(t)
	require.NotNil(t, claimed)
	assert.Equal(t, task.ref(), claimed.TaskId)
}

func TestPickupSkipsADuplicateOfAnIssueWithAnOpenPullRequest(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, true)
	other := seedTask(t, "other", "[github-issue:900003]\n\nDo it.", "in_review", nil)
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO factory.linked_pull_requests (url, task_id, state)
		VALUES ('https://example.test/pr/900003', $1, 'open')`, other.id)
	require.NoError(t, err)
	dup := seedTask(t, "dup", "[github-issue:900003]\n\nDo it.", "ready", nil)

	assert.Nil(t, env.claim(t))
	env.assertSkipped(t, dup, "already has an open job or pull request")
}

func TestPickupGoesPastASkippedTaskToTheNext(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, true)
	seedTask(t, "epic", "Do it.", "ready", []string{"epic"})
	next := seedTask(t, "next", "Do it.", "ready", nil)

	claimed := env.claim(t)
	require.NotNil(t, claimed)
	assert.Equal(t, next.ref(), claimed.TaskId)
}
