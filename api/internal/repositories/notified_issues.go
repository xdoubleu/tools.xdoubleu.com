package repositories

import (
	"context"

	"tools.xdoubleu.com/internal/database/postgres"
)

// NotifiedIssuesRepository tracks which Sentry issues / DigitalOcean failed
// deployments have already triggered a notification email, so a restart or a
// repeated job run doesn't re-send one already sent (issue #561).
type NotifiedIssuesRepository struct {
	db postgres.DB
}

func NewNotifiedIssuesRepository(db postgres.DB) *NotifiedIssuesRepository {
	return &NotifiedIssuesRepository{db: db}
}

// Exists reports whether key has already been notified.
func (r *NotifiedIssuesRepository) Exists(
	ctx context.Context,
	key string,
) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM global.notified_issues WHERE key = $1)",
		key,
	).Scan(&exists)
	return exists, err
}

// Insert records key as notified.
func (r *NotifiedIssuesRepository) Insert(ctx context.Context, key string) error {
	_, err := r.db.Exec(
		ctx,
		"INSERT INTO global.notified_issues (key) VALUES ($1) ON CONFLICT (key) DO NOTHING",
		key,
	)
	return err
}
