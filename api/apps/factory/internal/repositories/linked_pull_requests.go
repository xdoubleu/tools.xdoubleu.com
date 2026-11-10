package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
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
		INSERT INTO factory.linked_pull_requests (url, task_id, state, ci, review)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (url) DO UPDATE
		SET task_id = $2, state = $3, ci = $4, review = $5, checked_at = now()`,
		p.URL, p.TaskID, p.State, p.CI, p.Review)
	return postgres.PgxErrorToHTTPError(err)
}

// ByTask lists a task's linked pull requests, newest read first.
func (r *LinkedPullRequestsRepository) ByTask(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.LinkedPullRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT url, task_id, state, ci, review, checked_at
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
			&p.CheckedAt); err != nil {
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
		SELECT url, task_id, state, ci, review, checked_at
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
			&p.CheckedAt); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
