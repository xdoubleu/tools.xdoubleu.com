package runner

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// BundleInfoPlistForTest returns the Info.plist of a made-up bundle.
func BundleInfoPlistForTest() string {
	return strings.ReplaceAll(bundleInfoPlist, "__RELEASE__", "v1")
}

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
	u.inner.ExecutablePath = func() (string, error) { return path, nil }
}

// SetExecutableError makes an Updater fail to find its executable.
func (u *Updater) SetExecutableError(err error) {
	u.inner.ExecutablePath = func() (string, error) { return "", err }
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

// ChecksFailureForTest exposes checksFailure.
func ChecksFailureForTest(outcome string) error { return checksFailure(outcome) }

// PhaseTimes is phaseTimes, for tests.
type PhaseTimes struct{ phaseTimes }

// Agent sets the agent time; AddWatch and Summary forward.
func (p *PhaseTimes) AddWatch(b, g, e time.Time)     { p.addWatch(b, g, e) }
func (p *PhaseTimes) Summary(d time.Duration) string { return p.summary(d) }
func (p *PhaseTimes) SetAgent(d time.Duration)       { p.agent = d }

// FlakeProblemForTest runs the sandbox-fix diff check.
func FlakeProblemForTest(files []string) error {
	return flakeProblem(files)
}

// RetriesOf is a finished job's repeated attempts by cause.
func RetriesOf(e JobEnd) map[string]int { return e.Times.retries }

// NewPhaseTimings builds phase times for tests.
func NewPhaseTimings(
	agent, publish, ci, merge time.Duration,
	retries map[string]int,
) *phaseTimes {
	return &phaseTimes{
		agent: agent, gates: 0, publish: publish, ci: ci, merge: merge,
		retries: retries,
	}
}

// LeakSlotForTest counts a job slot no job holds.
func (r *JobRunner) LeakSlotForTest() { r.slots.Add(1) }

// EnvironmentFailure exposes environmentFailure to the tests.
func EnvironmentFailure(text string, setupOnly bool) string {
	return environmentFailure(text, setupOnly)
}
