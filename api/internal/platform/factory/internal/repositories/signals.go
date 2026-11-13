package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// FailureCounts are the counts behind the factory failure gauges.
type FailureCounts struct {
	RedPullRequests   int
	JobsWithoutPR     int
	UnverifiedOutcome int
	StuckPullRequests int
}

// FailureCounts counts open pull requests with failing CI, and the jobs that
// finished since the given time without a pull request or with an outcome that
// admits the work was not checked.
func (r *JobsRepository) FailureCounts(
	ctx context.Context,
	since, stuckBefore time.Time,
) (FailureCounts, error) {
	var c FailureCounts
	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM factory.linked_pull_requests
				WHERE state = 'open' AND ci = 'failing'),
			(SELECT count(*) FROM factory.jobs
				WHERE status = 'succeeded' AND pr_url = '' AND finished_at >= $1),
			(SELECT count(*) FROM factory.jobs
				WHERE finished_at >= $1
				AND (outcome ILIKE '%not verified%' OR outcome ILIKE '%could not run%')),
			(SELECT count(*) FROM factory.linked_pull_requests
				WHERE state = 'open' AND mergeable <> '' AND unmergeable_since < $2)`,
		since, stuckBefore).Scan(&c.RedPullRequests, &c.JobsWithoutPR,
		&c.UnverifiedOutcome, &c.StuckPullRequests)
	return c, postgres.PgxErrorToHTTPError(err)
}

// MergedBetween lists tasks with a merge commit that last changed in [from, to],
// the ones that should be live by now.
func (r *TasksRepository) MergedBetween(
	ctx context.Context,
	from, to time.Time,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, repo_id, status, merge_sha, updated_at FROM factory.tasks
		WHERE merge_sha <> '' AND status IN ('testing', 'done')
		AND updated_at BETWEEN $1 AND $2`, from, to)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	out := []models.Task{}
	for rows.Next() {
		var t models.Task
		err = rows.Scan(&t.ID, &t.RepoID, &t.Status, &t.MergeSHA, &t.UpdatedAt)
		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
