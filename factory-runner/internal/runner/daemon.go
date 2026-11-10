package runner

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
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
	// Control, when set, pauses the heartbeat and update checks and records what
	// the menu bar shows.
	Control *Control
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
		if !updating {
			if check, requested := due(opts, clk.now(), lastCheck); check {
				lastCheck = clk.now()
				updating = true
				go func() { updated <- tryUpdate(ctx, opts, log, requested) }()
			}
		}

		if opts.Control != nil && opts.Control.Quiet() {
			clk.sleep(ctx, minLoop)
			continue
		}

		started := clk.now()
		woken, err := beatOnce(ctx, hb, token, opts.Control)
		switch {
		case ctx.Err() == nil && woken:
			// A check was asked for: cut the held heartbeat short, not a failure.
			continue
		case err == nil:
			backoff = minLoop
			opts.Control.RecordBeat()
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

// beatOnce makes one heartbeat call, cut short when the owner asks for an update
// check; woken says whether it was.
func beatOnce(
	ctx context.Context,
	hb Heartbeater,
	token string,
	c *Control,
) (bool, error) {
	hbCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var fired atomic.Bool
	if c != nil {
		go func() {
			select {
			case <-c.wake:
				fired.Store(true)
				cancel()
			case <-hbCtx.Done():
			}
		}()
	}
	err := hb.Heartbeat(hbCtx, token)
	return fired.Load(), err
}

// due reports whether to check for an update now, and whether the owner asked
// for it (whose outcome is notified, unlike the hourly check's).
func due(opts Options, now, lastCheck time.Time) (bool, bool) {
	if opts.Updater == nil {
		return false, false
	}
	if opts.Control != nil && opts.Control.Paused() {
		opts.Control.refuseCheck("not checked: the runner is stopped")
		return false, false
	}
	if opts.Activity != nil && !opts.Activity.Idle() {
		if opts.Control != nil {
			opts.Control.deferCheck("waiting for the current job to finish")
		}
		return false, false
	}
	// A requested check is only consumed once it can run.
	requested := opts.Control != nil && opts.Control.takeCheck()
	return requested || now.Sub(lastCheck) >= UpdateEvery, requested
}

func tryUpdate(
	ctx context.Context,
	opts Options,
	log *slog.Logger,
	requested bool,
) bool {
	idle := func() bool { return opts.Activity == nil || opts.Activity.Idle() }
	if opts.Control != nil {
		opts.Control.RecordUpdate("checking")
	}
	replaced, err := opts.Updater.Update(ctx, idle)
	outcome := "no newer release"
	switch {
	case errors.Is(err, ErrUpdateSigningUnset):
		outcome = "disabled: this build has no signing key"
		log.Warn("self-update skipped", "reason", outcome)
	case err != nil:
		outcome = "failed: " + err.Error()
		log.Warn("self-update failed", "error", err)
	case replaced:
		outcome = "installed a newer release"
		log.Info("self-update installed a newer release")
	default:
		log.Info("self-update checked", "result", outcome)
	}
	if opts.Control != nil {
		opts.Control.RecordUpdate(outcome)
		switch {
		case replaced:
			opts.Control.Notify("Updated to a new release; restarting")
		case requested && err != nil:
			opts.Control.Notify("Update " + outcome)
		case requested:
			opts.Control.Notify("Factory Runner is up to date")
		}
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
