package legacyauth_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/legacyauth"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestMigrate_NoLegacySchema covers the common case every local/CI database
// hits: no auth_gotrue_legacy schema at all, so Migrate must be a cheap,
// error-free no-op. The shared test DB already has a real auth schema (from
// normal migrations) but never a legacy one, so it's a fine target as-is.
func TestMigrate_NoLegacySchema(t *testing.T) {
	db := testhelper.ConnectTestDB("postgres://postgres@localhost/postgres")
	defer db.Close()

	err := legacyauth.Migrate(context.Background(), logging.NewNopLogger(), db, nil)
	require.NoError(t, err)
}

// TestMigrate_NilSealerSkipsTOTPFactors covers the degrade-gracefully path:
// with no encryption key configured (sealer == nil), user rows still copy,
// but TOTP factor migration is skipped rather than panicking.
func TestMigrate_NilSealerSkipsTOTPFactors(t *testing.T) {
	ctx := context.Background()

	adminPool := testhelper.ConnectTestDB("postgres://postgres@localhost/postgres")
	defer adminPool.Close()

	dbName := fmt.Sprintf("legacyauth_niltest_%s", uuid.NewString()[:8])
	_, err := adminPool.Exec(ctx, "CREATE DATABASE "+dbName)
	require.NoError(t, err)
	defer func() {
		_, _ = adminPool.Exec(
			ctx, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", dbName),
		)
	}()

	scratchDSN := "postgres://postgres@localhost/" + dbName
	scratchPool := testhelper.ConnectTestDB(scratchDSN)
	defer scratchPool.Close()

	_, err = scratchPool.Exec(ctx, `
		CREATE SCHEMA auth_gotrue_legacy;
		CREATE TABLE auth_gotrue_legacy.users (
			id UUID PRIMARY KEY,
			email TEXT,
			encrypted_password TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE auth_gotrue_legacy.mfa_factors (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL,
			factor_type TEXT NOT NULL,
			status TEXT NOT NULL,
			secret TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE SCHEMA auth;
		CREATE TABLE auth.users (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE auth.totp_factors (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL,
			secret TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	require.NoError(t, err)

	userID := uuid.NewString()
	hash, err := bcrypt.GenerateFromPassword([]byte("whatever"), bcrypt.DefaultCost)
	require.NoError(t, err)

	_, err = scratchPool.Exec(ctx, `
		INSERT INTO auth_gotrue_legacy.users (id, email, encrypted_password)
		VALUES ($1, 'nil-sealer@example.com', $2)
	`, userID, string(hash))
	require.NoError(t, err)

	_, err = scratchPool.Exec(ctx, `
		INSERT INTO auth_gotrue_legacy.mfa_factors
			(id, user_id, factor_type, status, secret)
		VALUES ($1, $2, 'totp', 'verified', 'JBSWY3DPEHPK3PXP')
	`, uuid.NewString(), userID)
	require.NoError(t, err)

	err = legacyauth.Migrate(ctx, logging.NewNopLogger(), scratchPool, nil)
	require.NoError(t, err)

	var userCount, factorCount int
	require.NoError(t, scratchPool.QueryRow(
		ctx, "SELECT count(*) FROM auth.users",
	).Scan(&userCount))
	require.Equal(t, 1, userCount, "user row should still be copied with a nil sealer")

	require.NoError(t, scratchPool.QueryRow(
		ctx, "SELECT count(*) FROM auth.totp_factors",
	).Scan(&factorCount))
	require.Equal(
		t,
		0,
		factorCount,
		"TOTP factors must be skipped, not attempted, with a nil sealer",
	)
}
