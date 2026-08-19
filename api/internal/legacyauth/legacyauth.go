// Package legacyauth copies GoTrue's legacy auth data — renamed out of the
// way to auth_gotrue_legacy by migration 00017_auth_schema.sql's own
// detection logic — into the new self-hosted auth.users/auth.totp_factors
// tables (issue #1039).
//
// Migrate runs automatically as part of api's own boot sequence
// (Application.ApplyMigrations), under the same advisory lock that
// serializes migrations across concurrently-starting replicas, immediately
// after the global migrations (which create the new auth schema) apply. It
// is idempotent — every insert is ON CONFLICT DO NOTHING — so running it
// again on a database that's already been migrated, or one that never had
// a legacy schema at all (every local/CI database), is a cheap no-op.
package legacyauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/internal/crypto"
)

// Migrate copies users and verified TOTP factors out of auth_gotrue_legacy,
// if that schema exists. sealer may be nil (ENCRYPTION_KEY unset) — TOTP
// factor migration is then skipped, matching how the rest of the app
// already degrades when no encryption key is configured.
func Migrate(
	ctx context.Context,
	logger *slog.Logger,
	db *pgxpool.Pool,
	sealer *crypto.Sealer,
) error {
	exists, err := legacySchemaExists(ctx, db)
	if err != nil {
		return fmt.Errorf("checking for auth_gotrue_legacy schema: %w", err)
	}
	if !exists {
		logger.Info(
			"auth_gotrue_legacy schema not found — nothing to migrate " +
				"(expected on local/CI databases, and on any boot after " +
				"the one-time production cutover)",
		)
		return nil
	}

	usersCopied, err := migrateUsers(ctx, db)
	if err != nil {
		return fmt.Errorf("migrating legacy auth users: %w", err)
	}
	logger.Info("legacy auth: copied users", "count", usersCopied)

	if sealer == nil {
		logger.Warn(
			"ENCRYPTION_KEY not set — skipping legacy TOTP factor migration",
		)
		return nil
	}

	factorsCopied, err := migrateTOTPFactors(ctx, db, sealer)
	if err != nil {
		return fmt.Errorf("migrating legacy TOTP factors: %w", err)
	}
	logger.Info("legacy auth: copied verified TOTP factors", "count", factorsCopied)

	return nil
}

func legacySchemaExists(ctx context.Context, db *pgxpool.Pool) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.schemata
			WHERE schema_name = 'auth_gotrue_legacy'
		)
	`).Scan(&exists)
	return exists, err
}

// migrateUsers copies auth_gotrue_legacy.users(id, email, encrypted_password,
// created_at) into auth.users(id, email, password_hash, created_at).
// GoTrue's encrypted_password is already a bcrypt hash, so it's copied
// as-is — no re-hashing needed, only a column rename.
func migrateUsers(ctx context.Context, db *pgxpool.Pool) (int64, error) {
	tag, err := db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash, created_at)
		SELECT id, email, encrypted_password, created_at
		FROM auth_gotrue_legacy.users
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// migrateTOTPFactors copies verified TOTP factors from
// auth_gotrue_legacy.mfa_factors (factor_type='totp', status='verified')
// into auth.totp_factors, re-encrypting the secret through crypto.Sealer.Seal
// on the way in — GoTrue stores it in plaintext, the new schema never does.
func migrateTOTPFactors(
	ctx context.Context, db *pgxpool.Pool, sealer *crypto.Sealer,
) (int64, error) {
	rows, err := db.Query(ctx, `
		SELECT id, user_id, secret, created_at FROM auth_gotrue_legacy.mfa_factors
		WHERE factor_type = 'totp' AND status = 'verified'
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type legacyFactor struct {
		id        string
		userID    string
		secret    string
		createdAt any
	}

	var factors []legacyFactor
	for rows.Next() {
		var f legacyFactor
		if err = rows.Scan(&f.id, &f.userID, &f.secret, &f.createdAt); err != nil {
			return 0, err
		}
		factors = append(factors, f)
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}

	var copied int64
	for _, f := range factors {
		sealed, sealErr := sealer.Encrypt([]byte(f.secret))
		if sealErr != nil {
			return copied, fmt.Errorf(
				"encrypting secret for factor %s: %w",
				f.id,
				sealErr,
			)
		}

		tag, execErr := db.Exec(ctx, `
			INSERT INTO auth.totp_factors (id, user_id, secret, status, created_at)
			VALUES ($1, $2, $3, 'verified', $4)
			ON CONFLICT (id) DO NOTHING
		`, f.id, f.userID, base64.StdEncoding.EncodeToString(sealed), f.createdAt)
		if execErr != nil {
			return copied, fmt.Errorf("inserting factor %s: %w", f.id, execErr)
		}
		copied += tag.RowsAffected()
	}

	return copied, nil
}
