package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// advanceMain commits a file to the remote's default branch.
func advanceMain(t *testing.T, remote, name, content string) {
	t.Helper()
	work := t.TempDir()
	gitIn(t, work, "clone", remote, ".")
	require.NoError(t, os.WriteFile(filepath.Join(work, name), []byte(content), 0o644))
	gitIn(t, work, "add", ".")
	gitIn(t, work, "-c", "user.name=a", "-c", "user.email=a@example.com",
		"commit", "-m", "main moves")
	gitIn(t, work, "push", "origin", "HEAD")
}

func TestABranchBehindMainGetsMainMergedBeforeTheAgentAndGates(t *testing.T) {
	env := newEnv(t, laterJob(writes("First.", 0)), newJob())
	env.sandbox.realSync = true
	ranFirstJob(t, env)
	advanceMain(t, env.remote, "main-only.txt", "new")
	rerun := replyJob()
	rerun.ResumeSessionID = ""
	env.api.jobs = append(env.api.jobs, rerun)

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	clone := env.sandbox.specs[1].CloneDir
	assert.FileExists(t, filepath.Join(clone, "main-only.txt"), "main was merged")
	assert.FileExists(t, filepath.Join(clone, "agent.txt"), "the earlier work stays")
	assert.Equal(t, "", gitIn(t, clone, "status", "--porcelain"))
	assert.NotContains(t, env.sandbox.specs[len(env.sandbox.specs)-1].Stdin,
		"conflicts")
	assert.True(t, env.api.done()[1].succeeded)
}

func TestAConflictWithMainIsHandedToTheAgentWithTheTreeClean(t *testing.T) {
	env := newEnv(t, laterJob(writes("First.", 0)), newJob())
	env.sandbox.realSync = true
	ranFirstJob(t, env)
	advanceMain(t, env.remote, "agent.txt", "main's version")
	rerun := replyJob()
	rerun.ResumeSessionID = ""
	env.api.jobs = append(env.api.jobs, rerun)

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	agentPrompt := env.sandbox.specs[1].Stdin
	assert.Contains(t, agentPrompt, "merging origin/main")
	assert.Contains(t, agentPrompt, "conflicts")
	clone := env.sandbox.specs[1].CloneDir
	assert.NoFileExists(t, filepath.Join(clone, ".git", "MERGE_HEAD"))
}
