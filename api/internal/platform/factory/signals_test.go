package factory_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/githubapp"
)

func gaugeValue(t *testing.T, name string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() == name {
			return f.GetMetric()[0].GetGauge().GetValue()
		}
	}
	t.Fatalf("gauge %s is not registered", name)
	return 0
}

func TestTheFailureGaugesCountWhatNobodyPointsAt(t *testing.T) {
	env, gh := githubEnv(t)
	ctx := context.Background()
	// The database outlives runs: forget the commit an earlier run left behind.
	_, err := testDB.Exec(ctx,
		`UPDATE factory.tasks SET merge_sha = '' WHERE merge_sha = 'signal-sha'`)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx,
		`DELETE FROM factory.linked_pull_requests WHERE url = $1`, pullURL(51))
	require.NoError(t, err)
	require.NoError(t, env.app.CollectSignalsNow(ctx))
	red := gaugeValue(t, "factory_pull_requests_red")
	noPR := gaugeValue(t, "factory_jobs_finished_without_pr")
	unverified := gaugeValue(t, "factory_jobs_unverified_outcome")
	undeployed := gaugeValue(t, "factory_merged_tasks_undeployed")

	// A red open pull request.
	inReview(t, env, "red signal", 51)
	gh.answer(51, githubapp.PullRequestStatus{
		State: githubapp.StateOpen, CI: githubapp.CIFailing, Review: "", Mergeable: "", MergeSHA: "",
	}, nil)

	// A job that finished with no pull request, and one that was not verified.
	for _, outcome := range []string{"Did it.", "Changed it, not verified."} {
		task := env.newTask(t, "no pr "+outcome)
		env.run(t, task.Id, "")
		job := env.claim(t)
		_, err = env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
			JobId: job.Id, Succeeded: true, Outcome: outcome,
		})
		require.NoError(t, err)
	}

	// A task merged over an hour ago whose commit is not live.
	waiting := env.newTask(t, "waiting signal")
	env.putInReview(t, waiting.Id)
	gh.mergeAs(pullURL(8), "Task: waiting ("+waiting.Id+")\n", "signal-sha")
	require.NoError(t, env.poll(t))
	_, err = testDB.Exec(ctx, `
		INSERT INTO factory.linked_pull_requests (url, task_id, state, ci)
		SELECT $1, id, 'open', 'failing' FROM factory.tasks
		WHERE title = 'red signal'
		ON CONFLICT (url) DO UPDATE SET state = 'open', ci = 'failing'`, pullURL(51))
	require.NoError(t, err)
	_, err = testDB.Exec(ctx,
		`UPDATE factory.tasks SET updated_at = now() - interval '2 hours'
		WHERE merge_sha = 'signal-sha'`)
	require.NoError(t, err)

	require.NoError(t, env.app.CollectSignalsNow(ctx))

	assert.Greater(t, gaugeValue(t, "factory_pull_requests_red"), red)
	assert.GreaterOrEqual(t, gaugeValue(t, "factory_jobs_finished_without_pr"), noPR+2)
	assert.Greater(t, gaugeValue(t, "factory_jobs_unverified_outcome"), unverified)
	assert.Greater(t, gaugeValue(t, "factory_merged_tasks_undeployed"), undeployed)
}
