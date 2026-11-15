package runner_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestFreeBytesReadsTheVolume(t *testing.T) {
	free, err := runner.FreeBytes(t.TempDir())
	require.NoError(t, err)
	assert.NotZero(t, free)

	_, err = runner.FreeBytes("/no/such/dir")
	assert.Error(t, err)
}

func TestMakeRoomSweepsAndPrunesOnlyWhenTheDiskIsLow(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	done := jobDir(t, env, jobDone, doneTask)
	open := jobDir(t, env, jobOpen, openTask)
	env.api.finished = []string{doneTask}
	pruned := 0
	prune := func(context.Context) { pruned++ }

	env.runner.DiskFree = func() (uint64, error) { return runner.LowDiskBytes, nil }
	assert.False(t, env.runner.MakeRoom(context.Background(), prune))
	assert.DirExists(t, done, "room enough: nothing is touched")
	assert.Zero(t, pruned)

	env.runner.DiskFree = func() (uint64, error) { return 1 << 30, nil }
	assert.True(t, env.runner.MakeRoom(context.Background(), prune), "still low")
	assert.NoDirExists(t, done)
	assert.DirExists(t, open, "unfinished work is never deleted for room")
	assert.Equal(t, 1, pruned)

	assert.True(t, env.runner.MakeRoom(context.Background(), nil))
}

func TestMakeRoomIsClearWhenCleaningFreedTheSpace(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	dir := jobDir(t, env, jobDone, doneTask)
	env.api.finished = []string{doneTask}
	env.runner.DiskFree = func() (uint64, error) {
		if _, err := os.Stat(dir); err == nil {
			return 1 << 30, nil
		}
		return runner.LowDiskBytes, nil
	}
	assert.False(t, env.runner.MakeRoom(context.Background(), nil))
}
