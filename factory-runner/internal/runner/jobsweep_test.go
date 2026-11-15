package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	jobDone  = "bbbbbbbb-0000-0000-0000-000000000001"
	jobOpen  = "bbbbbbbb-0000-0000-0000-000000000002"
	jobOld   = "bbbbbbbb-0000-0000-0000-000000000003"
	jobFresh = "bbbbbbbb-0000-0000-0000-000000000004"
)

func jobDir(t *testing.T, env *jobEnv, id, task string) string {
	t.Helper()
	dir := filepath.Join(env.dir, "jobs", id)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "clone"), 0o755))
	if task != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "task"), []byte(task), 0o600))
	}
	return dir
}

func TestJobDirsOfDoneTasksAndStaleUnmarkedOnesAreDeleted(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	done := jobDir(t, env, jobDone, doneTask)
	open := jobDir(t, env, jobOpen, openTask)
	old := jobDir(t, env, jobOld, "")
	fresh := jobDir(t, env, jobFresh, "")
	past := time.Now().Add(-15 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(old, past, past))
	env.api.finished = []string{doneTask, otherTask}

	env.runner.CleanJobs(context.Background())

	assert.NoDirExists(t, done)
	assert.DirExists(t, open)
	assert.NoDirExists(t, old)
	assert.DirExists(t, fresh)
	require.Len(t, env.api.asked, 1)
	assert.ElementsMatch(t, []string{doneTask, openTask}, env.api.asked[0])
}

func TestAFailedQuestionKeepsJobDirs(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	done := jobDir(t, env, jobDone, doneTask)
	env.api.finishedErr = assert.AnError

	env.runner.CleanJobs(context.Background())

	assert.DirExists(t, done)
}
