package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

type LinkedPullRequestsRepository struct {
	db postgres.DB
}

// Upsert records what the poller read of a pull request that links a task.
func (r *LinkedPullRequestsRepository) Upsert(
	ctx context.Context,
	p models.LinkedPullRequest,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO factory.linked_pull_requests
			(url, task_id, state, ci, review, mergeable, unmergeable_since)
		VALUES ($1, $2, $3, $4, $5, $6,
			CASE WHEN $6 <> '' THEN now() END)
		ON CONFLICT (url, task_id) DO UPDATE
		SET state = $3, ci = $4, review = $5, mergeable = $6,
			unmergeable_since = CASE WHEN $6 = '' THEN NULL
				ELSE COALESCE(factory.linked_pull_requests.unmergeable_since, now()) END,
			checked_at = now()`,
		p.URL, p.TaskID, p.State, p.CI, p.Review, p.Mergeable)
	return postgres.PgxErrorToHTTPError(err)
}

// UpsertMany records several pull request reads in one statement. Each
// (url, task) pair must appear once.
func (r *LinkedPullRequestsRepository) UpsertMany(
	ctx context.Context,
	ps []models.LinkedPullRequest,
) error {
	if len(ps) == 0 {
		return nil
	}
	urls := make([]string, len(ps))
	taskIDs := make([]uuid.UUID, len(ps))
	states := make([]string, len(ps))
	cis := make([]string, len(ps))
	reviews := make([]string, len(ps))
	mergeables := make([]string, len(ps))
	for i, p := range ps {
		urls[i], taskIDs[i], states[i] = p.URL, p.TaskID, p.State
		cis[i], reviews[i], mergeables[i] = p.CI, p.Review, p.Mergeable
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO factory.linked_pull_requests
			(url, task_id, state, ci, review, mergeable, unmergeable_since)
		SELECT u.url, u.task_id, u.state, u.ci, u.review, u.mergeable,
			CASE WHEN u.mergeable <> '' THEN now() END
		FROM unnest($1::text[], $2::uuid[], $3::text[], $4::text[], $5::text[],
			$6::text[]) AS u(url, task_id, state, ci, review, mergeable)
		ON CONFLICT (url, task_id) DO UPDATE
		SET state = EXCLUDED.state, ci = EXCLUDED.ci, review = EXCLUDED.review,
			mergeable = EXCLUDED.mergeable,
			unmergeable_since = CASE WHEN EXCLUDED.mergeable = '' THEN NULL
				ELSE COALESCE(factory.linked_pull_requests.unmergeable_since, now()) END,
			checked_at = now()`,
		urls, taskIDs, states, cis, reviews, mergeables)
	return postgres.PgxErrorToHTTPError(err)
}

// ByTask lists a task's linked pull requests, newest read first.
func (r *LinkedPullRequestsRepository) ByTask(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.LinkedPullRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT url, task_id, state, ci, review, checked_at, mergeable
		FROM factory.linked_pull_requests
		WHERE task_id = $1
		ORDER BY checked_at DESC, url`, taskID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	out := []models.LinkedPullRequest{}
	for rows.Next() {
		var p models.LinkedPullRequest
		if err = rows.Scan(&p.URL, &p.TaskID, &p.State, &p.CI, &p.Review,
			&p.CheckedAt, &p.Mergeable); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Open lists the pull requests not yet merged or closed, the ones read longest
// ago first.
func (r *LinkedPullRequestsRepository) Open(
	ctx context.Context,
	limit int,
) ([]models.LinkedPullRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT url, task_id, state, ci, review, checked_at, mergeable
		FROM factory.linked_pull_requests
		WHERE state NOT IN ('merged', 'closed')
		ORDER BY checked_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	out := []models.LinkedPullRequest{}
	for rows.Next() {
		var p models.LinkedPullRequest
		if err = rows.Scan(&p.URL, &p.TaskID, &p.State, &p.CI, &p.Review,
			&p.CheckedAt, &p.Mergeable); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
