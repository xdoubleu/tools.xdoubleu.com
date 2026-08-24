package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

func clearWorkflowRuns(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.workflow_job_samples")
	require.NoError(t, err)
	_, err = testDB.Exec(t.Context(), "DELETE FROM global.workflow_run_samples")
	require.NoError(t, err)
}

func workflowRunSample(
	runID int64,
	branch, conclusion string,
) models.WorkflowRunSample {
	now := time.Now()
	return models.WorkflowRunSample{
		RunID:        runID,
		WorkflowName: "CI",
		Branch:       branch,
		Event:        "push",
		Conclusion:   conclusion,
		URL:          "https://github.com/x/y/actions/runs/1",
		DurationMs:   60_000,
		StartedAt:    now.Add(-time.Minute),
		CompletedAt:  now,
	}
}

func TestWorkflowRunsExistsAndInsertRun(t *testing.T) {
	clearWorkflowRuns(t)
	repo := repositories.NewWorkflowRunsRepository(testDB)

	exists, err := repo.Exists(t.Context(), 1)
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(
		t,
		repo.InsertRun(t.Context(), workflowRunSample(1, "main", "success")),
	)

	exists, err = repo.Exists(t.Context(), 1)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestWorkflowRunsInsertRunIsIdempotent(t *testing.T) {
	clearWorkflowRuns(t)
	repo := repositories.NewWorkflowRunsRepository(testDB)

	sample := workflowRunSample(1, "main", "success")
	require.NoError(t, repo.InsertRun(t.Context(), sample))
	require.NoError(t, repo.InsertRun(t.Context(), sample))

	stats, err := repo.WorkflowDurationStats(t.Context(), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(1), stats[0].RunCount)
}

func TestWorkflowRunsInsertAndListJobs(t *testing.T) {
	clearWorkflowRuns(t)
	repo := repositories.NewWorkflowRunsRepository(testDB)
	require.NoError(
		t,
		repo.InsertRun(t.Context(), workflowRunSample(1, "main", "success")),
	)

	now := time.Now()
	require.NoError(t, repo.InsertJobs(t.Context(), []models.WorkflowJobSample{
		{
			RunID: 1, JobName: "test", Conclusion: "success",
			DurationMs: 30_000, StartedAt: now.Add(-time.Minute), CompletedAt: now,
		},
	}))

	stats, err := repo.JobDurationStats(t.Context(), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, "test", stats[0].JobName)
	assert.InDelta(t, 30_000, stats[0].AvgDurationMs, 0.001)
}

func TestWorkflowRunsMainFailures(t *testing.T) {
	clearWorkflowRuns(t)
	repo := repositories.NewWorkflowRunsRepository(testDB)

	require.NoError(
		t,
		repo.InsertRun(t.Context(), workflowRunSample(1, "main", "success")),
	)
	require.NoError(
		t,
		repo.InsertRun(t.Context(), workflowRunSample(2, "main", "failure")),
	)
	require.NoError(
		t, repo.InsertRun(t.Context(), workflowRunSample(3, "feature", "failure")),
	)

	failures, err := repo.MainFailures(
		t.Context(), "main", "failure", time.Now().Add(-time.Hour),
	)
	require.NoError(t, err)
	require.Len(t, failures, 1)
	assert.Equal(t, int64(2), failures[0].RunID)
}

func TestWorkflowRunsWorkflowDurationStats(t *testing.T) {
	clearWorkflowRuns(t)
	repo := repositories.NewWorkflowRunsRepository(testDB)

	require.NoError(
		t,
		repo.InsertRun(t.Context(), workflowRunSample(1, "main", "success")),
	)
	require.NoError(
		t,
		repo.InsertRun(t.Context(), workflowRunSample(2, "main", "success")),
	)

	stats, err := repo.WorkflowDurationStats(t.Context(), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, "CI", stats[0].WorkflowName)
	assert.Equal(t, int64(2), stats[0].RunCount)
	assert.InDelta(t, 60_000, stats[0].AvgDurationMs, 0.001)
}

func TestWorkflowRunsPruneOlderThan(t *testing.T) {
	clearWorkflowRuns(t)
	repo := repositories.NewWorkflowRunsRepository(testDB)

	old := workflowRunSample(1, "main", "success")
	old.StartedAt = time.Now().AddDate(0, 0, -100)
	require.NoError(t, repo.InsertRun(t.Context(), old))
	require.NoError(t, repo.InsertJobs(t.Context(), []models.WorkflowJobSample{
		{
			RunID: 1, JobName: "test", Conclusion: "success",
			DurationMs: 1_000, StartedAt: old.StartedAt, CompletedAt: old.StartedAt,
		},
	}))
	require.NoError(
		t,
		repo.InsertRun(t.Context(), workflowRunSample(2, "main", "success")),
	)

	require.NoError(
		t, repo.PruneOlderThan(t.Context(), time.Now().AddDate(0, 0, -30)),
	)

	exists1, err := repo.Exists(t.Context(), 1)
	require.NoError(t, err)
	assert.False(t, exists1)

	exists2, err := repo.Exists(t.Context(), 2)
	require.NoError(t, err)
	assert.True(t, exists2)

	jobStats, err := repo.JobDurationStats(t.Context(), time.Now().AddDate(0, 0, -365))
	require.NoError(t, err)
	assert.Empty(t, jobStats)
}
