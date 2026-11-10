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
	homeDir, execPath string,
) string {
	return launchAgentPlist(homeDir, execPath)
}

// LoopForTest runs the job loop with a stubbed sleep and a ticking clock.
func (r *JobRunner) LoopForTest(
	ctx context.Context,
	sleep func(context.Context, time.Duration),
	now func() time.Time,
) error {
	return r.loop(ctx, clock{sleep: sleep, now: now})
}

// PullRequestTextForTest is the title and body of a job's pull request.
func PullRequestTextForTest(
	outcome string,
	job *ClaimedJob,
	server string,
) (string, string) {
	return pullRequestText(outcome, job, server)
}

// PushEnvForTest is the environment a push carries its token in.
func PushEnvForTest(token string) []string { return pushEnv(token) }

// NeutralizeForTest defuses mentions and closing keywords in agent text.
func NeutralizeForTest(text string) string { return neutralize(text) }

// ScrubForTest removes a token from text.
func ScrubForTest(text, token string) string { return scrub(text, token) }

// GitEnvForTest runs git with extra environment, as a push does.
func (g *Git) GitEnvForTest(
	ctx context.Context, dir string, env []string, args ...string,
) ([]byte, error) {
	return g.gitEnv(ctx, dir, env, args...)
}

// FreeSubnets lists the job subnets free of the given `netstat` routes.
func FreeSubnets(routes string, n int) []string {
	var out []string
	for _, p := range freeSubnets(routes, n) {
		out = append(out, p.String())
	}
	return out
}
