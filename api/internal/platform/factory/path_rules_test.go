package factory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

func (gh *fakeGitHub) changes(number int, files ...string) {
	gh.mu.Lock()
	defer gh.mu.Unlock()
	gh.files[number] = files
}

func (gh *fakeGitHub) failFiles(err error) {
	gh.mu.Lock()
	defer gh.mu.Unlock()
	gh.filesErr = err
}

// mergedLinkedTask merges pull request number linking a fresh task and polls.
func mergedLinkedTask(t *testing.T, env jobEnv, gh *fakeGitHub, number int) string {
	t.Helper()
	task := env.newTask(t, "pathy")
	env.putInReview(t, task.Id)
	gh.merge(pullURL(number), "Task: pathy ("+task.Id+")\n\nbody")
	_ = env.poll(t) // fails while the file lists do
	return task.Id
}

func TestAMergedPullRequestGoesToDoneByItsChangedFiles(t *testing.T) {
	cases := map[string][]string{
		"docs":      {"README.md", "docs/adr-0001.md"},
		"tests":     {"api/apps/books/foo_test.go", "x.test.ts", "a/b.test.tsx"},
		"ci":        {".github/workflows/main.yml", "tools/x.sh", "api/Makefile"},
		"factory":   {"factory-runner/main.go", "api/internal/platform/factory/app.go"},
		"container": {"factory-runner/Containerfile"},
		"mixed":     {"docs/a.md", "x/y_test.go", "factory-runner/z.go", ".github/ci.yml"},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			env, gh := githubEnv(t)
			gh.changes(7, files...)
			id := mergedLinkedTask(t, env, gh, 7)

			got := env.task(t, id)
			assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_DONE, got.Task.Status)
			require.Len(t, got.Comments, 1)
			assert.Contains(t, got.Comments[0].Body, "was merged: "+pullURL(7))
		})
	}
}

func TestAMergedPullRequestOfAnyOtherFileGoesToTesting(t *testing.T) {
	cases := map[string][]string{
		"web":        {"docs/a.md", "web/src/App.tsx"},
		"web test":   {"web/src/App.test.tsx"},
		"infra":      {"infra/README.md"},
		"deploy":     {"config/deploy.yml"},
		"migration":  {"api/apps/books/migrations/1_x.sql"},
		"migrations": {"migrations/x.md"},
		"app code":   {"api/apps/books/main.go", "docs/a.md"},
		"no files":   {},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			env, gh := githubEnv(t)
			gh.changes(7, files...)
			id := mergedLinkedTask(t, env, gh, 7)

			assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING,
				env.task(t, id).Task.Status)
		})
	}
}

func TestAFileListThatCannotBeReadSendsTheTaskToTesting(t *testing.T) {
	env, gh := githubEnv(t)
	gh.changes(7, "docs/a.md")
	gh.failFiles(errors.New("boom"))

	id := mergedLinkedTask(t, env, gh, 7)

	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING,
		env.task(t, id).Task.Status)
}

func TestTheBackfillMovesOnlyTheCoveredTestingTasksAndOnlyOnce(t *testing.T) {
	env, gh := githubEnv(t)
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.board_settings SET path_rules_backfilled = false`)
	require.NoError(t, err)
	gh.changes(7, "docs/a.md")
	gh.changes(8, "api/apps/books/main.go")
	// The file lists fail at first: both tasks land in Testing and the backfill
	// has to try again.
	gh.failFiles(errors.New("boom"))
	covered := mergedLinkedTask(t, env, gh, 7)
	other := mergedLinkedTask(t, env, gh, 8)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING,
		env.task(t, covered).Task.Status)

	gh.failFiles(nil)
	require.NoError(t, env.poll(t))

	got := env.task(t, covered)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_DONE, got.Task.Status)
	assert.Contains(t, got.Comments[len(got.Comments)-1].Body, "docs rule")
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING,
		env.task(t, other).Task.Status)

	// A person moves it back: the backfill has run and leaves it there.
	_, err = env.tasks.MoveTask(context.Background(), &factoryv1.MoveTaskRequest{
		Id: covered, Status: factoryv1.TaskStatus_TASK_STATUS_TESTING,
	})
	require.NoError(t, err)
	require.NoError(t, env.poll(t))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING,
		env.task(t, covered).Task.Status)
}
