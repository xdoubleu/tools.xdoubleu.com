package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeClockLimiter() (*attemptLimiter, *time.Time) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newAttemptLimiter()
	l.now = func() time.Time { return now }
	return l, &now
}

func TestAttemptLimiter_LocksAfterFreeFailures(t *testing.T) {
	l, now := fakeClockLimiter()

	for range lockoutFreeFailures {
		require.NoError(t, l.check("k"))
		l.fail("k")
	}
	require.NoError(t, l.check("k"), "free failures don't lock")

	l.fail("k")
	require.ErrorIs(t, l.check("k"), ErrTooManyAttempts)
	require.NoError(t, l.check("other"), "keys are independent")

	*now = now.Add(lockoutBase)
	require.NoError(t, l.check("k"), "lock expires after lockoutBase")

	l.fail("k")
	*now = now.Add(lockoutBase)
	require.ErrorIs(t, l.check("k"), ErrTooManyAttempts, "second lock doubles")
	*now = now.Add(lockoutBase)
	require.NoError(t, l.check("k"))
}

func TestAttemptLimiter_BackoffCapsAtMax(t *testing.T) {
	l, now := fakeClockLimiter()
	for range lockoutFreeFailures + 20 {
		l.fail("k")
	}
	*now = now.Add(lockoutMax - time.Second)
	require.ErrorIs(t, l.check("k"), ErrTooManyAttempts)
	*now = now.Add(time.Second)
	require.NoError(t, l.check("k"))
}

func TestAttemptLimiter_SuccessAndForgetReset(t *testing.T) {
	l, now := fakeClockLimiter()
	for range lockoutFreeFailures {
		l.fail("k")
	}
	l.succeed("k")
	l.fail("k")
	require.NoError(t, l.check("k"), "success clears the count")

	for range lockoutFreeFailures - 1 {
		l.fail("k")
	}
	*now = now.Add(lockoutForget + time.Second)
	l.fail("k")
	require.NoError(t, l.check("k"), "stale failures are forgotten")
}

func TestAttemptLimiter_PrunesStaleEntries(t *testing.T) {
	l, now := fakeClockLimiter()
	l.fail("stale")
	*now = now.Add(lockoutForget + time.Second)
	for i := range lockoutPruneSize {
		l.entries[string(rune(i))+"-fresh"] = &attemptEntry{
			failures: 1, lastFailure: *now, lockedUntil: time.Time{},
		}
	}
	l.fail("new")
	assert.NotContains(t, l.entries, "stale")
	assert.Contains(t, l.entries, "new")
}

func TestCheckFactorCode_LockedOut(t *testing.T) {
	//nolint:exhaustruct // only the limiter is read before the lockout returns
	service := &LocalService{attempts: newAttemptLimiter()}
	for range lockoutFreeFailures + 1 {
		service.attempts.fail("mfa:user")
	}
	factor := &TOTPFactor{UserID: "user"} //nolint:exhaustruct // only UserID is read
	valid, err := service.checkFactorCode(t.Context(), factor, "123456")
	require.ErrorIs(t, err, ErrTooManyAttempts)
	assert.False(t, valid)
}
