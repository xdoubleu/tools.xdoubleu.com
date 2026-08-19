// Command migrate-legacy-auth is a one-off, standalone data migration
// (issue #1039) that copies GoTrue's legacy auth data — restored from
// Supabase into a schema renamed to auth_gotrue_legacy as a manual,
// production-only step documented separately — into the new self-hosted
// auth.users/auth.totp_factors tables.
//
// It is never run as part of api boot: it takes its own DSN, checks whether
// auth_gotrue_legacy exists at all (local/CI databases never have it, so
// this is a safe no-op there), and exits.
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/internal/crypto"
)

func main() {
	os.Exit(mainWithExitCode())
}

// mainWithExitCode does the real work and returns an exit code, so every
// os.Exit call lives in exactly one place and the pgxpool.Pool's defer
// Close() (in run, called from here) always actually runs first.
func mainWithExitCode() int {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	dsn := flag.String(
		"dsn", os.Getenv("DB_DSN"), "Postgres DSN (or set DB_DSN)",
	)
	encryptionKey := flag.String(
		"encryption-key",
		os.Getenv("ENCRYPTION_KEY"),
		"base64-encoded 32-byte AES key used to re-encrypt TOTP "+
			"secrets (or set ENCRYPTION_KEY)",
	)
	flag.Parse()

	if *dsn == "" {
		logger.Error("no DSN provided: pass -dsn or set DB_DSN")
		return 1
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return 1
	}
	defer pool.Close()

	if err = run(ctx, logger, pool, *encryptionKey); err != nil {
		logger.Error("migration failed", "error", err)
		return 1
	}
	return 0
}

func run(
	ctx context.Context,
	logger *slog.Logger,
	pool *pgxpool.Pool,
	encryptionKey string,
) error {
	exists, err := legacySchemaExists(ctx, pool)
	if err != nil {
		return fmt.Errorf("checking for auth_gotrue_legacy schema: %w", err)
	}
	if !exists {
		logger.Info(
			"auth_gotrue_legacy schema not found — nothing to migrate " +
				"(expected on local/CI databases)",
		)
		return nil
	}

	usersCopied, err := migrateUsers(ctx, pool)
	if err != nil {
		return fmt.Errorf("migrating users: %w", err)
	}
	logger.Info("copied users", "count", usersCopied)

	if encryptionKey == "" {
		logger.Warn(
			"no encryption key provided — skipping TOTP factor migration " +
				"(pass -encryption-key or set ENCRYPTION_KEY to also copy these)",
		)
		return nil
	}

	sealer, err := crypto.New(encryptionKey)
	if err != nil {
		return fmt.Errorf("building sealer: %w", err)
	}

	factorsCopied, err := migrateTOTPFactors(ctx, pool, sealer)
	if err != nil {
		return fmt.Errorf("migrating TOTP factors: %w", err)
	}
	logger.Info("copied verified TOTP factors", "count", factorsCopied)

	return nil
}

func legacySchemaExists(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
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
func migrateUsers(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `
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
	ctx context.Context, pool *pgxpool.Pool, sealer *crypto.Sealer,
) (int64, error) {
	rows, err := pool.Query(ctx, `
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

		tag, execErr := pool.Exec(ctx, `
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
