package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// noPRFilter selects jobs that succeeded unexplained without a pull request.
const noPRFilter = `status = 'succeeded' AND pr_url = '' AND routine = ''
	AND finished_at >= $1
	AND outcome !~ '(^|\n)\s*(Already-fixed|Sandbox-fix):'`

// FailureCounts counts open pull requests with failing CI, and the jobs that
// finished since the given time without a pull request or with an outcome that
// admits the work was not checked. Routine jobs are skipped: they open no pull
// request by design, and so are jobs whose outcome deliberately ends without
// one (Already-fixed:, Sandbox-fix:). The tasks of the counted jobs are named.
func (r *JobsRepository) FailureCounts(
	ctx context.Context,
	since, stuckBefore time.Time,
) (models.FailureCounts, error) {
	var c models.FailureCounts
	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM factory.linked_pull_requests
				WHERE state = 'open' AND ci = 'failing'),
			(SELECT count(*) FROM factory.jobs
				WHERE `+noPRFilter+`),
			ARRAY(SELECT 'T-' || number FROM factory.tasks
				WHERE id IN (SELECT task_id FROM factory.jobs WHERE `+noPRFilter+`)
				ORDER BY number LIMIT 10),
			(SELECT count(*) FROM factory.jobs
				WHERE finished_at >= $1 AND routine = ''
				AND (outcome ILIKE '%not verified%' OR outcome ILIKE '%could not run%')),
			(SELECT count(*) FROM factory.linked_pull_requests
				WHERE state = 'open' AND mergeable <> '' AND unmergeable_since < $2)`,
		since, stuckBefore).Scan(&c.RedPullRequests, &c.JobsWithoutPR, &c.NoPRTasks,
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
