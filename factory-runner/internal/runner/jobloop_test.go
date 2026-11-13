package runner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const passAll = "\n\nChecks: lint=pass, test=pass, docs=pass"

// attempts is a sandbox that answers each run with the next script, the last one
// repeating. A script gets the 1-based attempt number and the run's spec.
func attempts(
	scripts ...func(n int, spec runner.SandboxSpec) int,
) func(context.Context, runner.SandboxSpec) (int, error) {
	n := 0
	return func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		n++
		return scripts[min(n, len(scripts))-1](n, spec), nil
	}
}

// agentRuns logs a session, optionally commits a file named after the attempt (a
// real new commit) and writes the outcome.
func agentRuns(outcome string, commit bool) func(int, runner.SandboxSpec) int {
	return func(n int, spec runner.SandboxSpec) int {
		_, _ = fmt.Fprintln(spec.Log, `{"type":"system","session_id":"sess-9"}`)
		if commit {
			commitFile(spec.CloneDir, fmt.Sprintf("agent-%d.txt", n), "work")
		}
		if outcome != "" {
			_ = os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"),
				[]byte(outcome), 0o644)
		}
		return 0
	}
}

func cloneHead(t *testing.T, p *publishing) func() string {
	t.Helper()
	return func() string {
		return gitIn(t, p.sandbox.specs[0].CloneDir, "rev-parse", "HEAD")
	}
}

func ciRed(names ...string) runner.PullStatus {
	st := runner.PullStatus{
		State: runner.PullOpen, HeadSHA: "", CI: runner.CIFailure, Failed: nil,
	}
	for _, n := range names {
		st.Failed = append(st.Failed, runner.CheckFailure{Name: n, Excerpt: n + " broke"})
	}
	return st
}

func TestAFailedGateIsRetriedInTheSameSessionUntilThePullRequestMerges(t *testing.T) {
	p := newPublishing(t, attempts(
		agentRuns("First try\n\nChecks: lint=pass, test=fail", true),
		agentRuns("Second try"+passAll, true),
	))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, p.sandbox.specs, 2)
	second := p.sandbox.specs[1]
	assert.Equal(t, runner.ClaudeCode{}.Resume("sess-9"), second.Command)
	assert.Contains(t, second.Stdin, "test fail")
	assert.NotContains(t, second.Stdin, "Commit your work locally", "no preamble again")

	got := p.api.done()
	require.Len(t, got, 1, "one job, one completion")
	assert.True(t, got[0].succeeded)
	assert.NotEmpty(t, got[0].prURL)
	assert.Equal(t, "Second try"+passAll, got[0].outcome)

	assert.Equal(t, []string{
		"Attempt 2: not published: not every check passed (test fail), retrying",
	}, p.api.noted())
	assert.Contains(t, strings.Join(p.api.logged(), "\n"), `"type":"runner"`)
	assert.Len(t, p.gh.opened, 1, "only the attempt that passes is opened")
}

func TestRedCIAfterThePullRequestOpensIsFedBackAndPushedAgain(t *testing.T) {
	p := newPublishing(t, attempts(
		agentRuns("Did it"+passAll, true),
		agentRuns("Fixed CI"+passAll, true),
	))
	p.gh.head = cloneHead(t, p)
	p.gh.statuses = []runner.PullStatus{
		{State: runner.PullOpen, HeadSHA: "", CI: runner.CIPending, Failed: nil},
		ciRed("api-test", "web-lint"),
		{State: runner.PullMerged, HeadSHA: "", CI: "", Failed: nil},
	}

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, p.sandbox.specs, 2)
	feedback := p.sandbox.specs[1].Stdin
	assert.Contains(t, feedback, "CI failed on api-test, web-lint")
	assert.Contains(t, feedback, "api-test broke")
	assert.Contains(t, feedback, "https://github.com/xdoubleu/tools.xdoubleu.com/pull/7")
	assert.Equal(t, []string{"Attempt 2: CI failed on api-test, web-lint, retrying"},
		p.api.noted())

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Equal(t, "Fixed CI"+passAll, got.outcome)
	assert.NotEmpty(t, p.remoteBranch(t))
}

func TestACIFailureOnAnOlderCommitIsIgnored(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns("Did it"+passAll, true)))
	stale := ciRed("api-test")
	stale.HeadSHA = "0000000000000000000000000000000000000000"
	p.gh.statuses = []runner.PullStatus{
		stale, {State: runner.PullMerged, HeadSHA: "", CI: "", Failed: nil},
	}
	p.gh.head = cloneHead(t, p)

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Len(t, p.sandbox.specs, 1, "a red check of a replaced commit is no retry")
	assert.True(t, p.api.done()[0].succeeded)
}

func TestTheSameFailureTwiceWithNoNewCommitStopsTheLoop(t *testing.T) {
	p := newPublishing(t, attempts(
		agentRuns("Did it"+passAll, true),
		agentRuns("Tried"+passAll, false),
	))
	p.gh.head = cloneHead(t, p)
	p.gh.statuses = []runner.PullStatus{ciRed("api-test")}

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 2)
	got := p.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "Stopped: no progress")
	assert.Contains(t, got.outcome, "CI failed on api-test")
	assert.NotEmpty(t, got.prURL, "the open pull request stays linked")
}

func TestARepeatedGateFailureWithNoNewCommitStopsTheLoop(t *testing.T) {
	p := newPublishing(t, attempts(
		agentRuns("Did it\n\nChecks: lint=pass, test=fail", true),
		agentRuns("Again\n\nChecks: lint=pass, test=fail", false),
	))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 2)
	got := p.api.done()[0]
	assert.Contains(t, got.outcome, "Stopped: no progress")
	assert.Empty(t, got.prURL)
	assert.Empty(t, p.gh.opened)
}

func TestADifferentFailureIsNotNoProgress(t *testing.T) {
	p := newPublishing(t, attempts(
		agentRuns("One\n\nChecks: lint=fail, test=pass", true),
		agentRuns("Two\n\nChecks: lint=pass, test=fail", true),
		agentRuns("Three"+passAll, true),
	))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 3)
	assert.True(t, p.api.done()[0].succeeded)
	assert.NotEmpty(t, p.api.done()[0].prURL)
}

func TestABlockedLineStopsTheLoopAtOnce(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns(
		"Could not run it\n\nChecks: lint=pass, test=fail\nBlocked: no module cache", true)))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 1)
	got := p.api.done()[0]
	assert.Equal(t, "no module cache", got.blocked)
	assert.Empty(t, p.api.noted())
}

func TestAPullRequestClosedWithoutMergingEndsTheJob(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns("Did it"+passAll, true)))
	p.gh.statuses = []runner.PullStatus{{
		State: runner.PullClosed, HeadSHA: "", CI: "", Failed: nil,
	}}

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "closed without merging")
	assert.NotEmpty(t, got.prURL)
}

func TestAJobStoppedFromTheBoardWhileWatchingItsPullRequestEnds(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns("Did it"+passAll, true)))
	p.gh.statuses = []runner.PullStatus{{
		State: runner.PullOpen, HeadSHA: "", CI: runner.CIPending, Failed: nil,
	}}
	p.gh.onRead = func() {
		p.api.mu.Lock()
		p.api.stop = true
		p.api.mu.Unlock()
	}

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "Stopped: the server no longer counts this job")
}

func TestARunnerShutdownWhileWatchingEndsTheJobAndTellsTheServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := newPublishing(t, attempts(agentRuns("Did it"+passAll, true)))
	p.gh.statuses = []runner.PullStatus{{
		State: runner.PullOpen, HeadSHA: "", CI: runner.CIPending, Failed: nil,
	}}
	p.gh.onRead = cancel

	_, err := p.runner.Next(ctx)
	require.NoError(t, err)
	assert.Contains(t, p.api.done()[0].outcome, "The runner shut down")
}

func TestAPullRequestThatNeverMergesIsGivenUpOn(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns("Did it"+passAll, true)))
	p.runner.MaxWatch = 30 * time.Millisecond
	p.gh.statuses = []runner.PullStatus{{
		State: runner.PullOpen, HeadSHA: "", CI: runner.CIPending, Failed: nil,
	}}

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "was not merged within")
	assert.Greater(t, p.gh.reads, 1)
}

func TestAFailedReadOfThePullRequestIsRetriedNotFatal(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns("Did it"+passAll, true)))
	p.gh.statusErrs = 3

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.True(t, p.api.done()[0].succeeded)
	assert.Equal(t, 4, p.gh.reads)
}

func TestAGateOnlyCIChecksStillOpensThePullRequest(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns(
		"Platform fix\n\nChecks: lint=pass, build-mac=not run (CI checks it)", true)))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 1)
	assert.NotEmpty(t, p.api.done()[0].prURL)
}

func TestAUsageLimitWaitsInTheSessionUntilTheResetThenResumes(t *testing.T) {
	reset := time.Now().Add(2 * time.Hour)
	p := newPublishing(t, attempts(
		func(_ int, spec runner.SandboxSpec) int {
			_, _ = fmt.Fprintln(spec.Log, `{"type":"system","session_id":"sess-9"}`)
			_, _ = fmt.Fprintln(spec.Log, rejectedAt(reset.Unix()))
			return 1
		},
		agentRuns("Done"+passAll, true),
	))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, p.sandbox.specs, 2)
	assert.Equal(t, runner.ClaudeCode{}.Resume("sess-9"), p.sandbox.specs[1].Command)
	assert.Contains(t, p.sandbox.specs[1].Stdin, "usage limit has lifted")

	waits := p.api.waits()
	require.NotEmpty(t, waits)
	assert.InDelta(t, float64(2*time.Hour), float64(waits[0]), float64(time.Minute))

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.False(t, got.limited)
	assert.Contains(t, strings.Join(p.api.noted(), "\n"), "usage limit reached")
}

func TestAUsageLimitWithoutAResetTimeWaitsAnHour(t *testing.T) {
	p := newPublishing(t, attempts(
		func(_ int, spec runner.SandboxSpec) int {
			_, _ = fmt.Fprintln(spec.Log, `{"type":"system","session_id":"sess-9"}`)
			_, _ = fmt.Fprintln(spec.Log,
				`{"type":"result","is_error":true,"result":"Claude AI usage limit reached"}`)
			return 1
		},
		agentRuns("Done"+passAll, true),
	))

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, float64(time.Hour), float64(p.api.waits()[0]), float64(time.Minute))
	assert.True(t, p.api.done()[0].succeeded)
}

func TestAJobWithoutAPublisherStillEndsAfterOneAttempt(t *testing.T) {
	env := newEnv(t, attempts(agentRuns("Did it\n\nChecks: test=fail", true)), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Len(t, env.sandbox.specs, 1, "nothing to publish to, nothing to retry")
	assert.True(t, env.api.done()[0].succeeded)
}

func TestTheAttemptLimitEndsALoopThatNeverGetsAnywhere(t *testing.T) {
	n := 0
	p := newPublishing(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		n++
		// A new commit each time, a different failure each time.
		return agentRuns(fmt.Sprintf("Try\n\nChecks: gate%d=fail", n), true)(n, spec), nil
	})

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.sandbox.specs, 20)
	assert.Contains(t, p.api.done()[0].outcome, "Stopped after 20 attempts")
}

func TestARetryNeedsASessionToResume(t *testing.T) {
	p := newPublishing(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		commitFile(spec.CloneDir, "a.txt", "x")
		_ = os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"),
			[]byte("Try\n\nChecks: test=fail"), 0o644)
		return 0, nil
	})

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Len(t, p.sandbox.specs, 1)
	assert.Contains(t, p.api.done()[0].outcome, "no session to resume")
}

func TestTheVMIsKeptForTheWholeJobWhenTheSandboxCanKeepIt(t *testing.T) {
	exec := newExecScript(routesAnswer)
	box := sandboxWith(t, exec)

	session, err := box.Open(context.Background(), spec(t, &bytes.Buffer{}))
	require.NoError(t, err)
	for _, in := range []string{"first", "again"} {
		code, execErr := session.Exec(context.Background(), []string{"claude"}, in,
			&bytes.Buffer{})
		require.NoError(t, execErr)
		assert.Zero(t, code)
	}
	killsBefore := countOf(exec.steps(), "kill")
	session.Close()

	steps := exec.steps()
	assert.Equal(t, 1, countOf(steps, "run"), "one VM")
	assert.Equal(t, 2, countOf(steps, "exec"))
	assert.Equal(t, killsBefore+1, countOf(steps, "kill"), "destroyed on Close")

	execs := exec.find("exec")
	assert.Contains(t, strings.Join(execs[0], " "), "npm ci", "setup runs first")
	assert.NotContains(t, strings.Join(execs[1], " "), "npm ci", "and once")
}

func TestAnUnreadableBranchTipDoesNotStopAPublishedJob(t *testing.T) {
	p := newPublishing(t, attempts(agentRuns("Did it"+passAll, true)))
	p.runner.Git.Run = func(
		ctx context.Context, dir string, args ...string,
	) ([]byte, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return nil, errors.New("no")
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		return cmd.CombinedOutput()
	}

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, p.api.done()[0].prURL)
}

func countOf(steps []string, name string) int {
	n := 0
	for _, s := range steps {
		if s == name {
			n++
		}
	}
	return n
}
