package auth

import (
	"errors"
	"sync"
	"time"
)

// ErrTooManyAttempts is returned while a key is locked out after repeated
// failed credential checks.
var ErrTooManyAttempts = errors.New("too many failed attempts, try again later")

const (
	lockoutFreeFailures = 5
	lockoutBase         = 30 * time.Second
	lockoutMax          = 15 * time.Minute
	// lockoutForget drops a key whose last failure is older than this.
	lockoutForget    = time.Hour
	lockoutPruneSize = 10_000
)

// attemptLimiter counts failed credential checks per key (an email or user ID)
// and, past lockoutFreeFailures, locks the key out for an exponentially
// growing period. It is per-process: it slows online guessing across IPs,
// which the per-IP rate limiter can't.
type attemptLimiter struct {
	mu      sync.Mutex
	entries map[string]*attemptEntry
	now     func() time.Time
}

type attemptEntry struct {
	failures    int
	lastFailure time.Time
	lockedUntil time.Time
}

func newAttemptLimiter() *attemptLimiter {
	return &attemptLimiter{
		mu:      sync.Mutex{},
		entries: map[string]*attemptEntry{},
		now:     time.Now,
	}
}

// check returns ErrTooManyAttempts while key is locked out.
func (l *attemptLimiter) check(key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.entries[key]; ok && l.now().Before(e.lockedUntil) {
		return ErrTooManyAttempts
	}
	return nil
}

// fail records a failed attempt for key.
func (l *attemptLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if len(l.entries) >= lockoutPruneSize {
		l.prune(now)
	}

	e, ok := l.entries[key]
	if !ok || now.Sub(e.lastFailure) > lockoutForget {
		e = &attemptEntry{failures: 0, lastFailure: now, lockedUntil: time.Time{}}
		l.entries[key] = e
	}
	e.failures++
	e.lastFailure = now

	if over := e.failures - lockoutFreeFailures; over > 0 {
		lock := lockoutMax
		if over <= 6 { //nolint:mnd // 30s << 5 already exceeds lockoutMax
			lock = min(lockoutBase<<(over-1), lockoutMax)
		}
		e.lockedUntil = now.Add(lock)
	}
}

// succeed clears key's failures.
func (l *attemptLimiter) succeed(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *attemptLimiter) prune(now time.Time) {
	for k, e := range l.entries {
		if now.Sub(e.lastFailure) > lockoutForget && now.After(e.lockedUntil) {
			delete(l.entries, k)
		}
	}
}
