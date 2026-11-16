package auth

import (
	"context"
	"errors"
	"log/slog"
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
	// lockoutMaxShift is the doubling count past which lockoutMax applies.
	lockoutMaxShift = 6
)

// lockDuration is how long a key with the given failure count stays locked;
// zero within the free failures.
func lockDuration(failures int) time.Duration {
	over := failures - lockoutFreeFailures
	if over <= 0 {
		return 0
	}
	if over > lockoutMaxShift { // 30s << 5 already exceeds lockoutMax
		return lockoutMax
	}
	return min(lockoutBase<<(over-1), lockoutMax)
}

// failureStore persists failure counts per key.
type failureStore interface {
	// Locked reports whether key is locked at now.
	Locked(ctx context.Context, key string, now time.Time) (bool, error)
	// RecordFailure atomically counts a failure and sets the lock.
	RecordFailure(ctx context.Context, key string, now time.Time) error
	Clear(ctx context.Context, key string) error
}

// attemptLimiter counts failed credential checks per key (an email or user ID)
// and, past lockoutFreeFailures, locks the key out for an exponentially
// growing period. It slows online guessing across IPs, which the per-IP rate
// limiter can't.
type attemptLimiter struct {
	store failureStore
	now   func() time.Time
}

func newAttemptLimiter(store failureStore) *attemptLimiter {
	return &attemptLimiter{store: store, now: time.Now}
}

// check returns ErrTooManyAttempts while key is locked out.
func (l *attemptLimiter) check(ctx context.Context, key string) error {
	locked, err := l.store.Locked(ctx, key, l.now())
	if err != nil {
		return err
	}
	if locked {
		return ErrTooManyAttempts
	}
	return nil
}

// fail records a failed attempt for key.
func (l *attemptLimiter) fail(ctx context.Context, key string) {
	if err := l.store.RecordFailure(ctx, key, l.now()); err != nil {
		slog.Default().ErrorContext(ctx, "failed to record login failure", "error", err)
	}
}

// succeed clears key's failures.
func (l *attemptLimiter) succeed(ctx context.Context, key string) {
	if err := l.store.Clear(ctx, key); err != nil {
		slog.Default().ErrorContext(ctx, "failed to clear login failures", "error", err)
	}
}

// memoryFailures is the in-process failureStore, used when the users store
// has no persistence of its own.
type memoryFailures struct {
	mu      sync.Mutex
	entries map[string]*attemptEntry
}

type attemptEntry struct {
	failures    int
	lastFailure time.Time
	lockedUntil time.Time
}

func newMemoryFailures() *memoryFailures {
	return &memoryFailures{mu: sync.Mutex{}, entries: map[string]*attemptEntry{}}
}

func (m *memoryFailures) Locked(
	_ context.Context, key string, now time.Time,
) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	return ok && now.Before(e.lockedUntil), nil
}

func (m *memoryFailures) RecordFailure(
	_ context.Context, key string, now time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.entries) >= lockoutPruneSize {
		for k, e := range m.entries {
			if now.Sub(e.lastFailure) > lockoutForget && now.After(e.lockedUntil) {
				delete(m.entries, k)
			}
		}
	}

	e, ok := m.entries[key]
	if !ok || now.Sub(e.lastFailure) > lockoutForget {
		e = &attemptEntry{failures: 0, lastFailure: now, lockedUntil: time.Time{}}
		m.entries[key] = e
	}
	e.failures++
	e.lastFailure = now
	if lock := lockDuration(e.failures); lock > 0 {
		e.lockedUntil = now.Add(lock)
	}
	return nil
}

func (m *memoryFailures) Clear(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}

// failurePruner deletes forgotten failure rows.
type failurePruner interface {
	PruneFailures(ctx context.Context, now time.Time) error
}

// LoginFailurePruneJob drops login_failures rows that are forgotten and no
// longer locked.
type LoginFailurePruneJob struct {
	repo failurePruner
}

func NewLoginFailurePruneJob(repo *Repository) *LoginFailurePruneJob {
	return &LoginFailurePruneJob{repo: repo}
}

func (j *LoginFailurePruneJob) ID() string {
	return "prune-login-failures"
}

func (j *LoginFailurePruneJob) RunEvery() time.Duration {
	return lockoutForget
}

func (j *LoginFailurePruneJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.repo.PruneFailures(ctx, time.Now())
}
