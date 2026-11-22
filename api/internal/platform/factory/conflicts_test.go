package factory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/githubapp"
)

func conflicting(head string) githubapp.PullRequestStatus {
	return githubapp.PullRequestStatus{
		State: githubapp.StateOpen, CI: githubapp.CIPassing, Review: "",
		Mergeable: githubapp.MergeConflicting, MergeSHA: "", HeadSHA: head,
	}
}

func TestAConflictingArmedPullRequestWithNoJobGetsAJobThatMergesTheBase(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "conflicts")
	env.run(t, task.Id)
	job := env.claim(t)
	complete(t, env, job.Id, true, pullURL(42))
	gh.openLinked(pullURL(42), 42, "Task: conflicts ("+task.Id+")")
	gh.open[0].Branch = job.Branch
	gh.answer(42, conflicting("head1"), nil)

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS, got.Task.Status)
	next := env.claim(t)
	require.NotNil(t, next)
	assert.Equal(t, job.Branch, next.Branch)
	assert.Contains(t, next.Prompt, "Merge origin/")
	assert.Contains(t, next.Prompt, "conflict:head1")

	// A job already queued or running is not doubled.
	require.NoError(t, env.poll(t))
	assert.Nil(t, env.claim(t))
}

func TestASecondConflictAtTheSameHeadParksTheTaskInNeedsYou(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "stubborn")
	env.run(t, task.Id)
	job := env.claim(t)
	complete(t, env, job.Id, true, pullURL(42))
	gh.openLinked(pullURL(42), 42, "Task: stubborn ("+task.Id+")")
	gh.open[0].Branch = job.Branch
	gh.answer(42, conflicting("head1"), nil)
	require.NoError(t, env.poll(t))
	complete(t, env, env.claim(t).Id, true, pullURL(42))

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.Contains(t, got.Comments[len(got.Comments)-1].Body, "conflict:head1")
	assert.Nil(t, env.claim(t))
}

func TestANewCommitThatConflictsAgainGetsAnotherJob(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "again")
	env.run(t, task.Id)
	job := env.claim(t)
	complete(t, env, job.Id, true, pullURL(42))
	gh.openLinked(pullURL(42), 42, "Task: again ("+task.Id+")")
	gh.open[0].Branch = job.Branch
	gh.answer(42, conflicting("head1"), nil)
	require.NoError(t, env.poll(t))
	complete(t, env, env.claim(t).Id, true, pullURL(42))
	gh.answer(42, conflicting("head2"), nil)

	require.NoError(t, env.poll(t))

	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS,
		env.task(t, task.Id).Task.Status)
	assert.NotNil(t, env.claim(t))
}
