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

// RunForTest runs the daemon loop with a stubbed sleep.
func RunForTest(
	ctx context.Context,
	hb Heartbeater,
	token string,
	log *slog.Logger,
	sleep func(context.Context, time.Duration),
) error {
	return run(ctx, hb, token, log, sleep)
}

// SleepForTest exposes the real sleep.
func SleepForTest(ctx context.Context, d time.Duration) { sleepCtx(ctx, d) }

// LaunchAgentPlistForTest exposes the plist renderer.
func LaunchAgentPlistForTest(
	execPath string,
) string {
	return launchAgentPlist(execPath)
}
