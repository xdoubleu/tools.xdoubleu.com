package objectstore

import (
	"context"
	"errors"
	"time"
)

// Retry parameters for DeleteWithRetry. Failures this targets are transient
// (network blip, rate limit) — a book/library delete that leaves its R2
// object behind because of one of these becomes a permanent orphan, only
// caught later by the daily storage scan.
const (
	deleteMaxAttempts = 4
	deleteBackoffCap  = 30 * time.Second
)

// pkg/unicat and pkg/hardcover's identical pattern.
//
//nolint:gochecknoglobals // overridden in tests via SetBackoffBase, mirroring
var deleteBackoffBase = 500 * time.Millisecond

// SetBackoffBase overrides the exponential-backoff base delay. Intended for
// tests only.
func SetBackoffBase(d time.Duration) { deleteBackoffBase = d }

// DeleteWithRetry calls store.Delete up to deleteMaxAttempts times with
// exponential backoff, returning the last error if every attempt fails.
// Mirrors the doWithRetry/backoffDelay shape already duplicated in
// pkg/unicat and pkg/hardcover, specialized for the one object-store
// operation whose failure silently leaks storage rather than just failing a
// request.
func DeleteWithRetry(ctx context.Context, store Client, key string) error {
	var lastErr error
	for i := range deleteMaxAttempts {
		err := store.Delete(ctx, key)
		if err == nil {
			return nil
		}

		if errors.Is(err, context.Canceled) {
			return err
		}

		lastErr = err

		if i == deleteMaxAttempts-1 {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(deleteBackoffDelay(i)):
		}
	}
	return lastErr
}

func deleteBackoffDelay(attempt int) time.Duration {
	delay := deleteBackoffBase
	for range attempt {
		delay *= 2
		if delay > deleteBackoffCap {
			return deleteBackoffCap
		}
	}
	return delay
}
