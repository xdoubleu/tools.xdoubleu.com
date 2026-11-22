package runner_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/wardcase/session"
)

// vmBox is a sandbox that keeps a VM, whose commands all succeed.
type vmBox struct{ *fakeSandbox }

type vmSession struct{}

func (vmSession) Exec(context.Context, []string, string, io.Writer) (int, error) {
	return 0, nil
}

func (vmSession) Close() {}

func (vmBox) Open(context.Context, runner.SandboxSpec) (runner.SandboxSession, error) {
	return vmSession{}, nil
}

// fakeSessions stands in for wardcase's session API, recording each run's options
// and request; act plays the harness.
type fakeSessions struct {
	mu   sync.Mutex
	opts []session.Options
	reqs []session.Request
	act  func(opts session.Options, req session.Request) session.Result
}

func (f *fakeSessions) factory(opts session.Options) runner.SessionAPI {
	return fakeSession{f: f, opts: opts}
}

type fakeSession struct {
	f    *fakeSessions
	opts session.Options
}

func (s fakeSession) Run(
	_ context.Context, req session.Request,
) (session.Result, error) {
	s.f.mu.Lock()
	s.f.opts = append(s.f.opts, s.opts)
	s.f.reqs = append(s.f.reqs, req)
	s.f.mu.Unlock()
	return s.f.act(s.opts, req), nil
}

func done(exit int) session.Result {
	return session.Result{
		ExitCode: exit, Harness: session.Claude, Continued: false, Limited: false,
		SessionID: "", Model: "", Branch: "", Path: "", Uncommitted: 0, Unpushed: 0,
	}
}

// finishes plays a harness that commits work and writes an outcome.
func finishes(t *testing.T) func(session.Options, session.Request) session.Result {
	t.Helper()
	return func(_ session.Options, req session.Request) session.Result {
		commitFile(req.Dir, "agent.txt", "work")
		require.NoError(t, os.WriteFile(filepath.Join(req.Extra.OutcomeDir, "outcome.md"),
			[]byte("Did it.\nPR title: Fix it"), 0o644))
		return done(0)
	}
}

func sessionEnv(
	t *testing.T, policy, model string, job *runner.ClaimedJob,
) (*jobEnv, *fakeSessions) {
	t.Helper()
	env := newEnv(t, writes("", 0), job)
	fake := &fakeSessions{mu: sync.Mutex{}, opts: nil, reqs: nil, act: finishes(t)}
	env.runner.Sandbox = vmBox{env.sandbox}
	env.runner.Sessions = fake.factory
	env.runner.OnLimit = func() (string, string) { return policy, model }
	env.runner.ClaudeToken = func() (string, error) { return "claude-token", nil }
	env.runner.Proxy = proxyOK
	env.runner.OpenRouterKey = func() (string, error) { return "or-key", nil }
	return env, fake
}

func TestAClaudeAttemptRunsInWardcaseOnTheJobsCloneAndBranch(t *testing.T) {
	env, fake := sessionEnv(t, runner.OnLimitWait, "", newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, fake.reqs, 1, env.api.done()[0].outcome)
	req := fake.reqs[0]
	assert.Equal(t, branch, req.Branch)
	assert.DirExists(t, req.Dir)
	assert.Equal(t, session.Claude, req.Harness)
	assert.Contains(t, req.Prompt, "# Task: Fix it")
	assert.Equal(t, "/outcome/outcome.md", req.Extra.Env["FACTORY_OUTCOME_PATH"])
	assert.Equal(t, jobID, req.Extra.Env["FACTORY_JOB"])
	assert.True(t, env.api.done()[0].succeeded)
	assert.Empty(t, env.sandbox.specs, "the agent did not run in the job's own VM")
}

func TestWaitMeansOverflowOffAndFallbackMeansAutoOnTheConfiguredModel(t *testing.T) {
	for name, tc := range map[string]struct {
		policy, model, overflow, fallback, openCode string
	}{
		"wait": {runner.OnLimitWait, "m/x", session.OverflowOff, "", ""},
		"fallback": {
			runner.OnLimitFallback, "m/x", session.OverflowAuto, session.OpenCode, "m/x",
		},
		"fallback no model": {runner.OnLimitFallback, "", session.OverflowOff, "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			env, fake := sessionEnv(t, tc.policy, tc.model, newJob())
			_, err := env.runner.Next(context.Background())
			require.NoError(t, err)
			req := fake.reqs[0]
			assert.Equal(t, tc.overflow, req.Overflow)
			assert.Equal(t, tc.fallback, req.Fallback)
			assert.Equal(t, tc.openCode, req.OpenCodeModel)
		})
	}
}

func TestAReplyResumesItsSessionAndNeverFallsBack(t *testing.T) {
	job := newJob()
	job.ResumeSessionID = "sess-1"
	env, fake := sessionEnv(t, runner.OnLimitFallback, "m/x", job)

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	req := fake.reqs[0]
	assert.Equal(t, "sess-1", req.ResumeID)
	assert.Equal(t, session.OverflowOff, req.Overflow)
	assert.Empty(t, req.Fallback)
}

func TestTheOpenRouterKeyReachesWardcaseOnlyAsAHostSecret(t *testing.T) {
	env, fake := sessionEnv(t, runner.OnLimitFallback, "m/x", newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	secrets := fake.opts[0].Secrets
	key, err := secrets.Get("openrouter")
	require.NoError(t, err)
	assert.Equal(t, "or-key", key)
	token, err := secrets.Get("anthropic")
	require.NoError(t, err)
	assert.Equal(t, "claude-token", token)
	_, err = secrets.Get("openai")
	require.Error(t, err)
	assert.NotContains(t, fake.reqs[0].Extra.Env, "OPENROUTER_API_KEY")
	for _, v := range fake.reqs[0].Extra.Env {
		assert.NotEqual(t, "or-key", v)
	}
}

func TestAUsageLimitWardcaseReportsStillRequeuesTheJob(t *testing.T) {
	env, fake := sessionEnv(t, runner.OnLimitWait, "", newJob())
	fake.act = func(opts session.Options, _ session.Request) session.Result {
		_, _ = io.WriteString(opts.Out, rejectedAt(4102444800)+"\n")
		res := done(1)
		res.Limited = true
		return res
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.True(t, got.limited)
	assert.False(t, got.succeeded)
}

func TestAContinuedSessionFinishesTheJobWithoutALimit(t *testing.T) {
	env, fake := sessionEnv(t, runner.OnLimitFallback, "m/x", newJob())
	finish := finishes(t)
	fake.act = func(opts session.Options, req session.Request) session.Result {
		_, _ = io.WriteString(opts.Out, rejectedAt(4102444800)+"\n")
		res := finish(opts, req)
		res.Continued, res.Harness = true, session.OpenCode
		return res
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.limited)
	assert.True(t, got.succeeded)
}

func TestAnOpenCodeJobStaysInItsOwnVMSession(t *testing.T) {
	env, fake := sessionEnv(t, runner.OnLimitWait, "", newJob())
	env.runner.Harness = runner.OpenCode{Model: "vendor/model"}
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Empty(t, fake.reqs)
}
