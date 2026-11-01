package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

type RunnersRepository struct {
	db postgres.DB
}

const runnerColumns = `id, name, created_by, created_at, pairing_expires_at,
	paired_at, last_seen, version, revoked_at`

func scanRunner(row rowScanner) (*models.Runner, error) {
	var r models.Runner
	err := row.Scan(
		&r.ID, &r.Name, &r.CreatedBy, &r.CreatedAt, &r.PairingExpiresAt,
		&r.PairedAt, &r.LastSeen, &r.Version, &r.RevokedAt,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &r, nil
}

// Create adds a runner waiting to pair with the code whose hash it is given.
// A runner that never paired and whose code ran out stops holding the one
// active slot; another active runner makes this a conflict.
func (r *RunnersRepository) Create(
	ctx context.Context,
	name, createdBy, pairingHash string,
	expiresAt time.Time,
) (*models.Runner, error) {
	if _, err := r.db.Exec(ctx, `
		UPDATE factory.runners SET revoked_at = now()
		WHERE revoked_at IS NULL AND token_hash IS NULL
		  AND pairing_expires_at < now()`); err != nil {
		return nil, err
	}
	return scanRunner(r.db.QueryRow(ctx, `
		INSERT INTO factory.runners
			(name, created_by, pairing_hash, pairing_expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING `+runnerColumns,
		name, createdBy, pairingHash, expiresAt,
	))
}

// Pair trades a valid, unused pairing code for the device token hash in one
// statement, so a code works once. An unknown, used, expired or revoked code is
// not found.
func (r *RunnersRepository) Pair(
	ctx context.Context,
	pairingHash, tokenHash, version string,
) (*models.Runner, error) {
	return scanRunner(r.db.QueryRow(ctx, `
		UPDATE factory.runners SET
			token_hash = $2, version = $3, paired_at = now(), last_seen = now(),
			pairing_hash = NULL, pairing_expires_at = NULL
		WHERE pairing_hash = $1 AND pairing_expires_at > now()
		  AND token_hash IS NULL AND revoked_at IS NULL
		RETURNING `+runnerColumns,
		pairingHash, tokenHash, version,
	))
}

// ByToken returns the active runner a device token hash belongs to.
func (r *RunnersRepository) ByToken(
	ctx context.Context,
	tokenHash string,
) (*models.Runner, error) {
	return scanRunner(r.db.QueryRow(ctx, `
		SELECT `+runnerColumns+` FROM factory.runners
		WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash))
}

// Touch records that the runner was heard from; a runner revoked meanwhile is
// not found. An empty version keeps the last one.
func (r *RunnersRepository) Touch(
	ctx context.Context,
	id uuid.UUID,
	version string,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE factory.runners
		SET last_seen = now(), version = COALESCE(NULLIF($2, ''), version)
		WHERE id = $1 AND revoked_at IS NULL`, id, version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return postgres.PgxErrorToHTTPError(pgx.ErrNoRows)
	}
	return nil
}

// List returns every runner, newest first, revoked ones included.
func (r *RunnersRepository) List(ctx context.Context) ([]models.Runner, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+runnerColumns+` FROM factory.runners
		ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Runner{}
	for rows.Next() {
		runner, scanErr := scanRunner(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *runner)
	}
	return out, rows.Err()
}

// Revoke ends a runner; its token stops authenticating at once. Revoking a
// revoked runner keeps its first revoke time.
func (r *RunnersRepository) Revoke(
	ctx context.Context,
	id uuid.UUID,
) (*models.Runner, error) {
	return scanRunner(r.db.QueryRow(ctx, `
		UPDATE factory.runners SET revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1
		RETURNING `+runnerColumns, id))
}
