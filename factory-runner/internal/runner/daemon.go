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
	// UpdateEvery is how often an idle runner looks for a newer release.
	UpdateEvery = time.Hour
)

var (
	// ErrRevoked ends the daemon: the server no longer accepts its token.
	ErrRevoked = errors.New("device token revoked: pair again with a new code")
	// ErrRestart ends the daemon after a self-update; launchd starts the new
	// binary.
	ErrRestart = errors.New("updated: restarting")
)

// Heartbeater is the server call the daemon repeats.
type Heartbeater interface {
	Heartbeat(ctx context.Context, token string) error
}

// Options are the daemon's optional parts. A nil Updater never self-updates; a
// nil Activity counts the runner as always idle.
type Options struct {
	Updater  SelfUpdater
	Activity *Activity
}

// clock is what the loop waits and reads time with, swapped in tests.
type clock struct {
	sleep func(context.Context, time.Duration)
	now   func() time.Time
}

// Run heartbeats until ctx ends, the token is revoked or a self-update replaced
// the binary. The server holds each call open, so a healthy loop makes one call
// per hold; a failing one backs off up to 30 seconds, and a fast answer is never
// repeated faster than once a second. While idle it checks for a newer release
// every UpdateEvery.
func Run(
	ctx context.Context,
	hb Heartbeater,
	token string,
	log *slog.Logger,
	opts Options,
) error {
	return run(ctx, hb, token, log, opts, clock{sleep: sleepCtx, now: time.Now})
}

func run(
	ctx context.Context,
	hb Heartbeater,
	token string,
	log *slog.Logger,
	opts Options,
	clk clock,
) error {
	backoff := minLoop
	var lastCheck time.Time
	// The update downloads for minutes at worst, so it runs beside the heartbeat
	// loop; its answer (replaced or not) arrives here.
	updated := make(chan bool, 1)
	updating := false
	for ctx.Err() == nil {
		select {
		case replaced := <-updated:
			updating = false
			if replaced {
				return ErrRestart
			}
		default:
		}
		if !updating && due(opts, clk.now(), lastCheck) {
			lastCheck = clk.now()
			updating = true
			go func() { updated <- tryUpdate(ctx, opts, log) }()
		}

		started := clk.now()
		err := hb.Heartbeat(ctx, token)
		switch {
		case err == nil:
			backoff = minLoop
			if wait := minLoop - clk.now().Sub(started); wait > 0 {
				clk.sleep(ctx, wait)
			}
		case ctx.Err() != nil:
			return nil //nolint:nilerr // a stop is not a failure
		case Unauthenticated(err):
			return ErrRevoked
		default:
			log.Warn("heartbeat failed", "error", err, "retry_in", backoff)
			clk.sleep(ctx, backoff)
			backoff = min(backoff*backoffFactor, maxBackoff)
		}
	}
	return nil
}

func due(opts Options, now, lastCheck time.Time) bool {
	if opts.Updater == nil || now.Sub(lastCheck) < UpdateEvery {
		return false
	}
	return opts.Activity == nil || opts.Activity.Idle()
}

func tryUpdate(ctx context.Context, opts Options, log *slog.Logger) bool {
	idle := func() bool { return opts.Activity == nil || opts.Activity.Idle() }
	replaced, err := opts.Updater.Update(ctx, idle)
	switch {
	case errors.Is(err, ErrUpdateSigningUnset):
		// A build without the key never updates; that is not a fault.
	case err != nil:
		log.Warn("self-update failed", "error", err)
	}
	return replaced
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}
