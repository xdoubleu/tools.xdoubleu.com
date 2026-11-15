package runner

import (
	"context"
	"log/slog"
	"time"
)

const (
	// WatchdogEvery is how often the watchdog compares the server's view with
	// the runner's own.
	WatchdogEvery = time.Minute
	// StallAfter is how long claimable work may wait while this runner could take
	// it before the watchdog reports and repairs.
	StallAfter = 10 * time.Minute
)

// Watchdog notices the symptom no unit test sees: jobs waiting on the board
// while this runner does nothing. It logs why and tries the repairs it owns.
type Watchdog struct {
	// State is the runner's claim state, with the server's queued count.
	State func() ClaimState
	// Reconcile frees job slots and busy counts that outlived their job.
	Reconcile func() int
	// KickJobs makes a runner whose jobs are off try to start them (the image
	// pull) now.
	KickJobs func()
	// Restart ends the daemon so launchd starts the runner afresh.
	Restart func()
	Log     *slog.Logger
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	since time.Time
	fixed time.Time
}

func (w *Watchdog) now() time.Time {
	if w.Now == nil {
		return time.Now()
	}
	return w.Now()
}

// Run checks every WatchdogEvery until ctx ends.
func (w *Watchdog) Run(ctx context.Context) {
	t := time.NewTicker(WatchdogEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Check()
		}
	}
}

// stalled says whether work waits that this runner could be taking: it has a
// free slot, or is draining with nothing left to drain, and the owner has not
// stopped it.
func stalled(s ClaimState) bool {
	if s.Queued <= 0 || s.Owner {
		return false
	}
	return s.Running < s.Max || (s.Draining && s.Running == 0)
}

// Check is one comparison. It reports whether it ran the repairs.
func (w *Watchdog) Check() bool {
	now := w.now()
	st := w.State()
	if !stalled(st) {
		w.since, w.fixed = time.Time{}, time.Time{}
		return false
	}
	if w.since.IsZero() {
		w.since = now
	}
	waited := now.Sub(w.since)
	if waited < StallAfter || (!w.fixed.IsZero() && now.Sub(w.fixed) < StallAfter) {
		return false
	}
	w.fixed = now
	reason := st.Reason
	if st.Claiming {
		reason = "claiming, but the server hands out nothing"
	}
	w.Log.Warn("stalled: jobs wait while this runner has room",
		"waiting_for", waited.Round(time.Second), "queued", st.Queued,
		"reason", reason, "running", st.Running, "max", st.Max, "jobs", st.Jobs,
		"draining", st.Draining, "jobs_off", st.JobsOff)

	freed := w.Reconcile()
	if freed > 0 {
		w.Log.Warn("watchdog freed leaked job slots", "freed", freed)
		st = w.State()
	}
	if st.JobsOff != "" {
		w.Log.Warn("watchdog retries starting the jobs", "error", st.JobsOff)
		w.KickJobs()
	}
	// Freed slots may be all the drain waited for; the next window judges that.
	if st.Draining && st.Running == 0 && freed == 0 {
		w.Log.Warn("watchdog restarts: the drain is stuck")
		w.Restart()
	}
	return true
}
