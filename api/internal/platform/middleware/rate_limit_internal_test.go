package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testSweepEvery  = time.Minute
	testRemoveAfter = 3 * time.Minute
)

// fakeClockIPLimiter has no refill and a bucket of one, so a client's second
// request is rejected for as long as its entry survives.
func fakeClockIPLimiter(removeAfter time.Duration) (*ipLimiter, *time.Time) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	return newIPLimiter(0, 1, testSweepEvery, removeAfter, func() time.Time {
		return now
	}), &now
}

func TestIPLimiter_ForgetsClientsIdleLongerThanRemoveAfter(t *testing.T) {
	t.Parallel()

	l, now := fakeClockIPLimiter(testRemoveAfter)

	assert.True(t, l.allow("a"))
	assert.False(t, l.allow("a"))

	*now = now.Add(testRemoveAfter + time.Second)
	assert.True(t, l.allow("b"))

	assert.Len(t, l.clients, 1, "a was swept")
	assert.True(t, l.allow("a"), "a starts over with a fresh bucket")
}

func TestIPLimiter_KeepsClientsSeenWithinRemoveAfter(t *testing.T) {
	t.Parallel()

	l, now := fakeClockIPLimiter(testRemoveAfter)

	assert.True(t, l.allow("a"))

	*now = now.Add(testRemoveAfter - time.Second)
	assert.True(t, l.allow("b"))

	assert.False(t, l.allow("a"), "a is still limited")
}

func TestIPLimiter_SweepsOnlyEverySweepEvery(t *testing.T) {
	t.Parallel()

	l, now := fakeClockIPLimiter(time.Second)

	assert.True(t, l.allow("a"))

	*now = now.Add(testSweepEvery / 2)
	assert.True(t, l.allow("b"))
	assert.Len(t, l.clients, 2, "idle a survives until the next sweep")

	*now = now.Add(testSweepEvery)
	assert.True(t, l.allow("c"))
	assert.Len(t, l.clients, 1, "a and b were swept")
}
