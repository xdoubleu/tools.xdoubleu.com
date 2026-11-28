package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// envProbeEvery is how often an open breaker runs the preflight again.
const (
	envProbeEvery = 30 * time.Second
	// sqliteHeader is how much of the Nix database the preflight reads.
	sqliteHeader = 16
)

// envSignatures name the failures of the runner's own environment: they come
// before the harness does any task work, so every job would meet them. The label
// is stable, so the board files one task per cause.
var envSignatures = []struct { //nolint:gochecknoglobals // a constant table
	label   string
	pattern *regexp.Regexp
}{
	{"the Nix store is unreadable", regexp.MustCompile(
		`(?i)disk I/O error|Device not configured|input/output error|` +
			`'nix': No such device`)},
	{"nix is missing in the VM", regexp.MustCompile(
		`(?i)nix: (command )?not found|'nix': No such file`)},
	{"the VM cannot reach its image or mounts", regexp.MustCompile(
		`(?i)no space left on device|read-only file system|` +
			`error mounting|failed to pull image`)},
}

// environmentFailure names the environment fault text shows, "" for none.
func environmentFailure(text string) string {
	for _, s := range envSignatures {
		if s.pattern.MatchString(text) {
			return s.label
		}
	}
	return ""
}

// envBlocked is the Blocked line for an environment fault: the same text for the
// same cause, so the server files one task and adds to it.
func envBlocked(signature string) string {
	return "Runner environment: " + signature
}

// holdEnv opens the breaker: the runner claims nothing until its preflight
// passes again. The first signature stays.
func (r *JobRunner) holdEnv(signature string) {
	r.env.CompareAndSwap(nil, &signature)
	r.envProbed.Store(r.now().UnixNano())
}

// envFault is why the breaker is open; empty when it is closed.
func (r *JobRunner) envFault() string {
	if p := r.env.Load(); p != nil {
		return *p
	}
	return ""
}

// envReady runs the preflight before a claim and says whether to claim. An open
// breaker is probed at most every envProbeEvery and closes when the preflight
// passes (or when there is none, to let a job tell).
func (r *JobRunner) envReady(ctx context.Context) bool {
	open := r.env.Load() != nil
	now := r.now()
	if open && now.UnixNano()-r.envProbed.Load() < int64(envProbeEvery) {
		return false
	}
	if err := r.preflight(ctx); err != nil {
		r.holdPreflight(err, open, now)
		return false
	}
	if open {
		r.Log.Info("environment is back, claims resume")
		r.env.Store(nil)
	}
	return true
}

func (r *JobRunner) preflight(ctx context.Context) error {
	if r.Preflight == nil {
		return nil
	}
	return r.Preflight(ctx)
}

// holdPreflight keeps the breaker open on a failed preflight; the first reason
// stays.
func (r *JobRunner) holdPreflight(err error, wasOpen bool, now time.Time) {
	if !wasOpen {
		r.Log.Warn("preflight failed, claims held", "error", err)
		reason := "preflight failed: " + err.Error()
		r.env.Store(&reason)
	}
	r.envProbed.Store(now.UnixNano())
}

// NixStorePreflight checks that the shared Nix store's directory and database
// can be read, which is what a dead mount breaks.
func NixStorePreflight(dir string) func(context.Context) error {
	return func(context.Context) error {
		if _, err := os.ReadDir(dir); err != nil {
			return fmt.Errorf("nix store %s: %w", dir, err)
		}
		db := filepath.Join(dir, "var", "nix", "db", "db.sqlite")
		f, err := os.Open(db)
		if errors.Is(err, os.ErrNotExist) {
			return nil // not seeded yet
		}
		if err != nil {
			return fmt.Errorf("nix database: %w", err)
		}
		defer f.Close()
		if _, err = io.ReadFull(f, make([]byte, sqliteHeader)); err != nil {
			return fmt.Errorf("nix database: %w", err)
		}
		return nil
	}
}

// environmentEnd is the step for an attempt that died on the runner's own
// environment before the agent worked: the job is handed back for requeueing and
// the breaker opens, so no other task is used up on it.
func (l *jobLoop) environmentEnd(signature string, res result) *step {
	l.r.holdEnv(signature)
	res.limited = true
	res.blocked = envBlocked(signature)
	res.outcome = strings.TrimSpace("The runner's environment failed before the " +
		"agent started: " + signature + ".\n\nThe runner holds its claims until " +
		"its preflight passes; the task goes back to the queue untouched.\n\n" +
		res.outcome)
	end := over(res)
	return &end
}
