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
// A setup signature comes only from the VM's setup script, so it counts even
// after the harness started, as long as the attempt did no work.
var envSignatures = []struct { //nolint:gochecknoglobals // a constant table
	label   string
	setup   bool
	pattern *regexp.Regexp
}{
	{"the Nix store is unreadable", true, regexp.MustCompile(
		`(?i)'nix': No such device|Device not configured|` +
			`wardcase: seeding /nix from \S+ failed|wardcase: /nix is not empty`)},
	{"the Nix store is unreadable", false, regexp.MustCompile(
		`(?i)disk I/O error|input/output error`)},
	{"nix is missing in the VM", true, regexp.MustCompile(
		`(?i)'nix': No such file`)},
	{"nix is missing in the VM", false, regexp.MustCompile(
		`(?i)nix: (command )?not found`)},
	{"the VM cannot reach its image or mounts", false, regexp.MustCompile(
		`(?i)no space left on device|read-only file system|` +
			`error mounting|failed to pull image`)},
}

// environmentFailure names the environment fault text shows, "" for none; with
// setupOnly, only the setup script's signatures count.
func environmentFailure(text string, setupOnly bool) string {
	for _, s := range envSignatures {
		if (s.setup || !setupOnly) && s.pattern.MatchString(text) {
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

// NixStorePreflight checks that the shared Nix store's directory, database and
// store/ can be read, which is what a dead mount breaks. A dead mount lists empty
// with no error, so the first healthy check leaves a marker beside dir (on the
// host's own disk, which the mount cannot take with it): from then on a missing
// database or an empty store/ fails instead of passing as not seeded yet.
func NixStorePreflight(dir string) func(context.Context) error {
	marker := filepath.Clean(dir) + ".seeded"
	return func(context.Context) error {
		if _, err := os.ReadDir(dir); err != nil {
			return fmt.Errorf("nix store %s: %w", dir, err)
		}
		_, err := os.Stat(marker)
		seeded := err == nil
		if err = nixStoreReadable(dir); errors.Is(err, os.ErrNotExist) && !seeded {
			return nil // not seeded yet
		}
		if err != nil {
			return err
		}
		if !seeded {
			_ = os.WriteFile(marker, nil, 0o600)
		}
		return nil
	}
}

// nixStoreReadable reads the database's header and lists store/; a fresh
// directory has neither, which comes back as os.ErrNotExist.
func nixStoreReadable(dir string) error {
	f, err := os.Open(filepath.Join(dir, "var", "nix", "db", "db.sqlite"))
	if err != nil {
		return fmt.Errorf("nix database: %w", err)
	}
	defer f.Close()
	if _, err = io.ReadFull(f, make([]byte, sqliteHeader)); err != nil {
		return fmt.Errorf("nix database: %w", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "store"))
	if err != nil {
		return fmt.Errorf("nix store: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("nix store: store/ is empty: %w", os.ErrNotExist)
	}
	return nil
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
