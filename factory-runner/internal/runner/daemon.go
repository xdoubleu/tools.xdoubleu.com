package runner

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	minLoop       = time.Second
	maxBackoff    = 30 * time.Second
	backoffFactor = 2
)

// ErrRevoked ends the daemon: the server no longer accepts its token.
var ErrRevoked = errors.New("device token revoked: pair again with a new code")

// Heartbeater is the server call the daemon repeats.
type Heartbeater interface {
	Heartbeat(ctx context.Context, token string) error
}

// Run heartbeats until ctx ends or the token is revoked. The server holds each
// call open, so a healthy loop makes one call per hold; a failing one backs off
// up to 30 seconds, and a fast answer is never repeated faster than once a
// second. now and sleep are injected for tests.
func Run(ctx context.Context, hb Heartbeater, token string, log *slog.Logger) error {
	return run(ctx, hb, token, log, sleepCtx)
}

func run(
	ctx context.Context,
	hb Heartbeater,
	token string,
	log *slog.Logger,
	sleep func(context.Context, time.Duration),
) error {
	backoff := minLoop
	for ctx.Err() == nil {
		started := time.Now()
		err := hb.Heartbeat(ctx, token)
		switch {
		case err == nil:
			backoff = minLoop
			if wait := minLoop - time.Since(started); wait > 0 {
				sleep(ctx, wait)
			}
		case ctx.Err() != nil:
			return nil //nolint:nilerr // a stop is not a failure
		case Unauthenticated(err):
			return ErrRevoked
		default:
			log.Warn("heartbeat failed", "error", err, "retry_in", backoff)
			sleep(ctx, backoff)
			backoff = min(backoff*backoffFactor, maxBackoff)
		}
	}
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}
