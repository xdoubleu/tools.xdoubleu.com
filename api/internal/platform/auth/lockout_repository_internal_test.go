package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/testhelper"
)

func newDBLimiter(t *testing.T) (*attemptLimiter, *Repository, *time.Time) {
	t.Helper()
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	repo := NewRepository(db)
	now := time.Now().UTC().Truncate(time.Second)
	l := newAttemptLimiter(repo)
	l.now = func() time.Time { return now }
	return l, repo, &now
}

func TestAttemptLimiter_DBLockoutSurvivesNewInstance(t *testing.T) {
	ctx := t.Context()
	l, repo, now := newDBLimiter(t)
	key := "signin:" + uuid.NewString()

	for range lockoutFreeFailures {
		l.fail(ctx, key)
	}
	require.NoError(t, l.check(ctx, key))
	l.fail(ctx, key)

	restarted := newAttemptLimiter(repo)
	restarted.now = l.now
	require.ErrorIs(t, restarted.check(ctx, key), ErrTooManyAttempts)

	*now = now.Add(lockoutBase)
	require.NoError(t, restarted.check(ctx, key), "lock expires")

	l.fail(ctx, key)
	*now = now.Add(lockoutBase)
	require.ErrorIs(t, l.check(ctx, key), ErrTooManyAttempts, "second lock doubles")

	l.succeed(ctx, key)
	require.NoError(t, l.check(ctx, key))
}

func TestAttemptLimiter_DBBackoffCapsAndForgets(t *testing.T) {
	ctx := t.Context()
	l, _, now := newDBLimiter(t)
	key := "mfa:" + uuid.NewString()

	for range lockoutFreeFailures + 20 {
		l.fail(ctx, key)
	}
	*now = now.Add(lockoutMax - time.Second)
	require.ErrorIs(t, l.check(ctx, key), ErrTooManyAttempts)
	*now = now.Add(time.Second)
	require.NoError(t, l.check(ctx, key))

	*now = now.Add(lockoutForget + time.Second)
	l.fail(ctx, key)
	require.NoError(t, l.check(ctx, key), "stale failures are forgotten")
}

func TestAttemptLimiter_DBConcurrentFailuresCount(t *testing.T) {
	ctx := t.Context()
	l, _, _ := newDBLimiter(t)
	key := "signin:" + uuid.NewString()

	var wg sync.WaitGroup
	for range lockoutFreeFailures + 1 {
		wg.Go(func() { l.fail(ctx, key) })
	}
	wg.Wait()
	require.ErrorIs(t, l.check(ctx, key), ErrTooManyAttempts)
}

func TestRepository_PruneFailures(t *testing.T) {
	ctx := t.Context()
	l, repo, now := newDBLimiter(t)
	stale, locked := "signin:"+uuid.NewString(), "signin:"+uuid.NewString()

	l.fail(ctx, stale)
	for range lockoutFreeFailures + 1 {
		l.fail(ctx, locked)
	}
	later := now.Add(lockoutForget + time.Second)
	require.NoError(t, repo.PruneFailures(ctx, later))

	var count int
	require.NoError(t, repo.db.QueryRow(ctx,
		`SELECT count(*) FROM auth.login_failures WHERE key = $1`, stale,
	).Scan(&count))
	assert.Zero(t, count)

	require.NoError(t, repo.db.QueryRow(ctx,
		`SELECT count(*) FROM auth.login_failures WHERE key = $1`, locked,
	).Scan(&count))
	assert.Zero(t, count, "an expired lock is pruned too")
}
