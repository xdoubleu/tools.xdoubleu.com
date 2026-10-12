// Package repositories stores the taste app's profiles.
package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

type ProfilesRepository struct {
	db postgres.DB
}

func NewProfilesRepository(db postgres.DB) *ProfilesRepository {
	return &ProfilesRepository{db: db}
}

// Get returns userID's profile, or nil when none exists yet.
func (r *ProfilesRepository) Get(
	ctx context.Context,
	userID string,
) (*models.Profile, error) {
	p := models.Profile{UserID: userID} //nolint:exhaustruct // scanned below
	err := r.db.QueryRow(ctx, `
		SELECT sections, model, generated_at, requested_at, pending, last_error
		FROM taste.profiles WHERE user_id = $1`, userID,
	).Scan(
		&p.Sections, &p.Model, &p.GeneratedAt, &p.RequestedAt, &p.Pending,
		&p.LastError,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // no profile yet is not an error
	}
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &p, nil
}

// Request marks userID's profile pending as of at, unless a request since
// notBefore exists; it reports whether it did.
func (r *ProfilesRepository) Request(
	ctx context.Context,
	userID string,
	at, notBefore time.Time,
) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		INSERT INTO taste.profiles (user_id, requested_at, pending)
		VALUES ($1, $2, true)
		ON CONFLICT (user_id) DO UPDATE
		SET requested_at = EXCLUDED.requested_at, pending = true
		WHERE taste.profiles.requested_at IS NULL
		   OR taste.profiles.requested_at < $3`, userID, at, notBefore)
	if err != nil {
		return false, postgres.PgxErrorToHTTPError(err)
	}
	return tag.RowsAffected() == 1, nil
}

// keepRequestedSince keeps a profile pending when it was requested after
// generation started ($2), so that request isn't consumed by older output.
const keepRequestedSince = `pending = COALESCE(taste.profiles.requested_at > $2, false)`

// SaveGenerated stores a profile generated from startedAt and clears any
// error.
func (r *ProfilesRepository) SaveGenerated(
	ctx context.Context,
	userID string,
	sections models.ProfileSections,
	model string,
	startedAt time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO taste.profiles
		    (user_id, sections, model, generated_at, attempted_at)
		VALUES ($1, $3, $4, $2, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET sections = EXCLUDED.sections, model = EXCLUDED.model,
		    generated_at = EXCLUDED.generated_at,
		    attempted_at = EXCLUDED.attempted_at, last_error = '',
		    `+keepRequestedSince, userID, startedAt, sections, model)
	return postgres.PgxErrorToHTTPError(err)
}

// SaveError notes a failed generation started at startedAt, keeping the
// previous profile.
func (r *ProfilesRepository) SaveError(
	ctx context.Context,
	userID, note string,
	startedAt time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO taste.profiles (user_id, last_error, attempted_at)
		VALUES ($1, $3, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET last_error = EXCLUDED.last_error,
		    attempted_at = EXCLUDED.attempted_at,
		    `+keepRequestedSince, userID, startedAt, note)
	return postgres.PgxErrorToHTTPError(err)
}

// SaveEmpty clears the profile with note when there was nothing to write it
// from, so no profile outlives the data it came from.
func (r *ProfilesRepository) SaveEmpty(
	ctx context.Context,
	userID, note string,
	startedAt time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO taste.profiles (user_id, last_error, attempted_at)
		VALUES ($1, $3, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET sections = NULL, model = '', generated_at = NULL,
		    last_error = EXCLUDED.last_error,
		    attempted_at = EXCLUDED.attempted_at,
		    `+keepRequestedSince, userID, startedAt, note)
	return postgres.PgxErrorToHTTPError(err)
}

// Due returns the users among userIDs whose profile is pending, or last
// attempted (failed or not) before staleBefore or never.
func (r *ProfilesRepository) Due(
	ctx context.Context,
	userIDs []string,
	staleBefore time.Time,
) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id
		FROM unnest($1::text[]) AS u (id)
		LEFT JOIN taste.profiles p ON p.user_id = u.id
		WHERE p.user_id IS NULL OR p.pending
		   OR p.attempted_at IS NULL OR p.attempted_at < $2
		ORDER BY u.id`, userIDs, staleBefore)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()
	var due []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		due = append(due, id)
	}
	return due, postgres.PgxErrorToHTTPError(rows.Err())
}
