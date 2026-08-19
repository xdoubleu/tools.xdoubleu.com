package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/legacyauth"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestApplyMigrations_MigratesLegacyGoTrueSchema is the critical test for
// issue #1039's automatic cutover: on a database shaped like production's
// (a GoTrue-owned `auth` schema, detected via `auth.instances`), booting the
// app must rename it to auth_gotrue_legacy, create the new auth.* tables
// fresh, and copy the existing user's password hash and verified TOTP
// factor across byte-for-byte/re-encrypted — all automatically, with no
// manual step. It also proves the whole thing is idempotent: running the
// copy again must not duplicate rows or error.
func TestApplyMigrations_MigratesLegacyGoTrueSchema(t *testing.T) {
	ctx := context.Background()

	adminPool := testhelper.ConnectTestDB("postgres://postgres@localhost/postgres")
	defer adminPool.Close()

	dbName := fmt.Sprintf("legacyauth_test_%s", uuid.NewString()[:8])
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

	legacyUserID := uuid.NewString()
	legacyFactorID := uuid.NewString()
	const legacyPassword = "correct horse battery staple"
	const legacyTOTPSecret = "JBSWY3DPEHPK3PXP"

	legacyHash, err := bcrypt.GenerateFromPassword(
		[]byte(legacyPassword), bcrypt.DefaultCost,
	)
	require.NoError(t, err)

	// Minimal shape of the real Supabase/GoTrue-owned auth schema — just
	// enough for the rename-detection (auth.instances) and the two tables
	// legacyauth.Migrate reads from.
	_, err = scratchPool.Exec(ctx, `
		CREATE SCHEMA auth;
		CREATE TABLE auth.instances (id UUID PRIMARY KEY);
		CREATE TABLE auth.users (
			id UUID PRIMARY KEY,
			email TEXT,
			encrypted_password TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE auth.mfa_factors (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL,
			factor_type TEXT NOT NULL,
			status TEXT NOT NULL,
			secret TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	require.NoError(t, err)

	_, err = scratchPool.Exec(ctx, `
		INSERT INTO auth.users (id, email, encrypted_password)
		VALUES ($1, 'legacy@example.com', $2)
	`, legacyUserID, string(legacyHash))
	require.NoError(t, err)

	_, err = scratchPool.Exec(ctx, `
		INSERT INTO auth.mfa_factors (id, user_id, factor_type, status, secret)
		VALUES ($1, $2, 'totp', 'verified', $3)
	`, legacyFactorID, legacyUserID, legacyTOTPSecret)
	require.NoError(t, err)

	cfg := testhelper.NewTestConfig()
	cfg.DBDsn = scratchDSN
	cfg.EncryptionKey = base64.StdEncoding.EncodeToString(make([]byte, 32))

	// Exercises the real production boot path end to end: global migrations
	// (including 00017's rename-detection), every app's own migrations, and
	// the legacyauth.Migrate call — all under the same advisory lock a real
	// deploy uses.
	app := NewApplication(logging.NewNopLogger(), cfg, scratchPool)
	require.NotNil(t, app)

	var legacySchemaExists bool
	err = scratchPool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.schemata
			WHERE schema_name = 'auth_gotrue_legacy'
		)
	`).Scan(&legacySchemaExists)
	require.NoError(t, err)
	require.True(t, legacySchemaExists, "legacy auth schema should have been renamed")

	var copiedHash string
	err = scratchPool.QueryRow(
		ctx, "SELECT password_hash FROM auth.users WHERE id = $1", legacyUserID,
	).Scan(&copiedHash)
	require.NoError(t, err)
	require.NoError(
		t,
		bcrypt.CompareHashAndPassword([]byte(copiedHash), []byte(legacyPassword)),
		"copied password hash must still verify the original password — "+
			"GoTrue's bcrypt hash is copied byte-for-byte, never re-hashed",
	)

	var copiedSecretB64, status string
	err = scratchPool.QueryRow(
		ctx,
		"SELECT secret, status FROM auth.totp_factors WHERE id = $1",
		legacyFactorID,
	).Scan(&copiedSecretB64, &status)
	require.NoError(t, err)
	require.Equal(t, "verified", status)

	sealer, err := crypto.New(cfg.EncryptionKey)
	require.NoError(t, err)
	sealedBytes, err := base64.StdEncoding.DecodeString(copiedSecretB64)
	require.NoError(t, err)
	decrypted, err := sealer.Decrypt(sealedBytes)
	require.NoError(t, err)
	require.Equal(
		t, legacyTOTPSecret, string(decrypted),
		"copied TOTP secret must decrypt back to GoTrue's original plaintext secret",
	)

	// Idempotency: running the copy again (as every subsequent boot does)
	// must not duplicate rows or error, even though auth_gotrue_legacy is
	// still present.
	err = legacyauth.Migrate(ctx, logging.NewNopLogger(), scratchPool, sealer)
	require.NoError(t, err)

	var userCount, factorCount int
	err = scratchPool.QueryRow(ctx, "SELECT count(*) FROM auth.users").
		Scan(&userCount)
	require.NoError(t, err)
	require.Equal(t, 1, userCount)

	err = scratchPool.QueryRow(ctx, "SELECT count(*) FROM auth.totp_factors").
		Scan(&factorCount)
	require.NoError(t, err)
	require.Equal(t, 1, factorCount)
}
