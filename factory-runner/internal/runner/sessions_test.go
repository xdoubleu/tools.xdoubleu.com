package runner_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionDirs makes a session directory for each name under the env's data dir.
func sessionDirs(t *testing.T, env *jobEnv, names ...string) string {
	t.Helper()
	root := filepath.Join(env.dir, "sessions")
	for _, name := range names {
		require.NoError(t, os.MkdirAll(filepath.Join(root, name, "projects"), 0o755))
	}
	return root
}

const (
	doneTask  = "aaaaaaaa-0000-0000-0000-000000000001"
	openTask  = "aaaaaaaa-0000-0000-0000-000000000002"
	otherTask = "aaaaaaaa-0000-0000-0000-000000000003"
)

func TestSessionsOfDoneTasksAreDeletedAndTheRestKept(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	root := sessionDirs(t, env, doneTask, openTask, "not-a-task")
	require.NoError(t, os.WriteFile(filepath.Join(root, otherTask), nil, 0o600))
	env.api.finished = []string{doneTask}

	env.runner.CleanSessions(context.Background())

	assert.NoDirExists(t, filepath.Join(root, doneTask))
	assert.DirExists(t, filepath.Join(root, openTask))
	assert.DirExists(t, filepath.Join(root, "not-a-task"))
	assert.FileExists(t, filepath.Join(root, otherTask), "a file is not a session")
	require.Len(t, env.api.asked, 1)
	assert.ElementsMatch(t, []string{doneTask, openTask}, env.api.asked[0])
}

func TestTheServerCannotNameADirectoryItWasNotAsked(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	root := sessionDirs(t, env, openTask, "keep")
	env.api.finished = []string{"keep", "../keep", otherTask, openTask + "/projects"}

	env.runner.CleanSessions(context.Background())

	assert.DirExists(t, filepath.Join(root, openTask, "projects"))
	assert.DirExists(t, filepath.Join(root, "keep"))
}

func TestNoSessionsMeansNoQuestion(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	env.runner.CleanSessions(context.Background())
	assert.Empty(t, env.api.asked, "no sessions directory")

	sessionDirs(t, env, "not-a-task")
	env.runner.CleanSessions(context.Background())
	assert.Empty(t, env.api.asked, "nothing that looks like a task")
}

func TestAFailedQuestionDeletesNothing(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	root := sessionDirs(t, env, doneTask)
	env.api.finishedErr = errors.New("server down")

	env.runner.CleanSessions(context.Background())

	assert.DirExists(t, filepath.Join(root, doneTask))
}

func TestManySessionsAreAskedAboutInBatchesOf500(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	names := make([]string, 501)
	for i := range names {
		names[i] = fmt.Sprintf("bbbbbbbb-0000-0000-0000-%012d", i)
	}
	sessionDirs(t, env, names...)

	env.runner.CleanSessions(context.Background())

	require.Len(t, env.api.asked, 2)
	assert.Len(t, env.api.asked[0], 500)
	assert.Len(t, env.api.asked[1], 1)
}

func TestSessionsAreCleanedAfterEachJob(t *testing.T) {
	env := newEnv(t, writes("ok", 0), newJob())
	root := sessionDirs(t, env, doneTask)
	env.api.finished = []string{doneTask}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	assert.NoDirExists(t, filepath.Join(root, doneTask))
	assert.DirExists(t, filepath.Join(root, taskID), "the job's own task is not finished")
}
