package runner

import (
	"context"
	"log/slog"
	"time"
)

// NewKeychainWith returns a Keychain whose `security` calls go to run.
func NewKeychainWith(run func(args ...string) ([]byte, int, error)) *Keychain {
	return &Keychain{run: run}
}

// RunForTest runs the daemon loop with a stubbed sleep and a clock that advances
// only by what the loop sleeps.
func RunForTest(
	ctx context.Context,
	hb Heartbeater,
	token string,
	log *slog.Logger,
	opts Options,
	sleep func(context.Context, time.Duration),
	clk func() time.Time,
) error {
	return run(ctx, hb, token, log, opts, clock{sleep: sleep, now: clk})
}

// NewCaffeinateWith returns a Caffeinate with its process starter replaced.
func NewCaffeinateWith(start func(pid int) (func(), error)) *Caffeinate {
	return &Caffeinate{start: start}
}

// SetExecutable points an Updater at another executable.
func (u *Updater) SetExecutable(path string) {
	u.executablePath = func() (string, error) { return path, nil }
}

// SetExecutableError makes an Updater fail to find its executable.
func (u *Updater) SetExecutableError(err error) {
	u.executablePath = func() (string, error) { return "", err }
}

// SleepForTest exposes the real sleep.
func SleepForTest(ctx context.Context, d time.Duration) { sleepCtx(ctx, d) }

// LaunchAgentPlistForTest exposes the plist renderer.
func LaunchAgentPlistForTest(
	execPath string,
) string {
	return launchAgentPlist(execPath)
}
