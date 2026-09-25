package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestApplyMigrations_LockTimeout: a held advisory lock fails fast instead of
// hanging.
func TestApplyMigrations_LockTimeout(t *testing.T) {
	ctx := context.Background()

	holderConn, err := testApp.db.Acquire(ctx)
	require.NoError(t, err)
	defer holderConn.Release()

	_, err = holderConn.Exec(
		ctx, "SELECT pg_advisory_lock($1)", migrationLockKey,
	)
	require.NoError(t, err)
	defer func() {
		_, _ = holderConn.Exec(
			ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey,
		)
	}()

	originalTimeout := migrationLockTimeout
	migrationLockTimeout = 50 * time.Millisecond
	defer func() { migrationLockTimeout = originalTimeout }()

	err = testApp.ApplyMigrations(testApp.db)

	require.ErrorContains(t, err, "failed to acquire migration lock")
}
