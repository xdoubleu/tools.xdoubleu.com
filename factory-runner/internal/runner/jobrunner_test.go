package runner_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const (
	jobID  = "11111111-2222-3333-4444-555555555555"
	branch = "agent/22222222-3333-4444-5555-666666666666-fix-it"
)

// gitIn runs git in dir with a fixed identity, failing the test on error.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git",
		append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com",
			"-c", "init.defaultBranch=main", "-c", "protocol.file.allow=always"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// newRemote is a bare repo with one commit on main, standing in for GitHub.
func newRemote(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	require.NoError(t, os.MkdirAll(work, 0o755))
	gitIn(t, work, "init")
	require.NoError(t, os.WriteFile(filepath.Join(work, "README"), []byte("hi"), 0o644))
	gitIn(t, work, "add", ".")
	gitIn(t, work, "commit", "-m", "first")
	bare := filepath.Join(root, "remote.git")
	gitIn(t, root, "clone", "--bare", work, bare)
	return bare
}

type fakeAPI struct {
	mu        sync.Mutex
	jobs      []*runner.ClaimedJob
	claimErr  error
	stop      bool
	checkIns  []string
	completed []completion
	completes chan struct{}
	events    []runner.EventLine
	appendErr error
	batches   int
	// pushToken and pushErr answer GetPushToken; tokenAsks counts the calls.
	pushToken string
	pushErr   error
	tokenAsks int
}

type completion struct {
	jobID     string
	succeeded bool
	outcome   string
	session   string
	prURL     string
}

func newFakeAPI(jobs ...*runner.ClaimedJob) *fakeAPI {
	return &fakeAPI{
		mu: sync.Mutex{}, jobs: jobs, claimErr: nil, stop: false, checkIns: nil,
		completed: nil, completes: make(chan struct{}, 10),
		events: nil, appendErr: nil, batches: 0,
		pushToken: "", pushErr: nil, tokenAsks: 0,
	}
}

func (f *fakeAPI) ClaimJob(context.Context, string) (*runner.ClaimedJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	if len(f.jobs) == 0 {
		return nil, nil //nolint:nilnil // no job is an answer
	}
	job := f.jobs[0]
	f.jobs = f.jobs[1:]
	return job, nil
}

func (f *fakeAPI) CheckIn(_ context.Context, _, _, session string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkIns = append(f.checkIns, session)
	return f.stop, nil
}

// AppendJobEvents stores lines like the server: a seq it has is ignored.
func (f *fakeAPI) AppendJobEvents(
	_ context.Context, _, _ string, lines []runner.EventLine,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches++
	if f.appendErr != nil {
		return f.appendErr
	}
	for _, l := range lines {
		seen := func(e runner.EventLine) bool { return e.Seq == l.Seq }
		if !slices.ContainsFunc(f.events, seen) {
			f.events = append(f.events, l)
		}
	}
	return nil
}

func (f *fakeAPI) numbered() []runner.EventLine {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]runner.EventLine(nil), f.events...)
}

func (f *fakeAPI) logged() []string {
	lines := []string{}
	for _, e := range f.numbered() {
		lines = append(lines, e.Line)
	}
	return lines
}

func (f *fakeAPI) GetPushToken(context.Context, string, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenAsks++
	return f.pushToken, f.pushErr
}

func (f *fakeAPI) CompleteJob(
	_ context.Context, _, id string, ok bool, outcome, session, prURL string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completed = append(f.completed, completion{id, ok, outcome, session, prURL})
	f.completes <- struct{}{}
	return nil
}

func (f *fakeAPI) done() []completion {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]completion(nil), f.completed...)
}

type fakeSandbox struct {
	run   func(ctx context.Context, spec runner.SandboxSpec) (int, error)
	specs []runner.SandboxSpec
}

func (f *fakeSandbox) Run(ctx context.Context, spec runner.SandboxSpec) (int, error) {
	f.specs = append(f.specs, spec)
	return f.run(ctx, spec)
}

// writes is a sandbox run that logs a session id and writes an outcome.
func writes(
	outcome string,
	exit int,
) func(context.Context, runner.SandboxSpec) (int, error) {
	return func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, `{"type":"system","session_id":"sess-9"}`+"\n")
		commitFile(spec.CloneDir, "agent.txt", "work")
		if outcome != "" {
			_ = os.WriteFile(
				filepath.Join(spec.OutcomeDir, "outcome.md"),
				[]byte(outcome),
				0o644,
			)
		}
		return exit, nil
	}
}

// commitFile commits one file in dir, as the agent would. Errors are ignored: a
// test whose clone is broken on purpose still gets its outcome file.
func commitFile(dir, name, content string) {
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=a",
			"-c", "user.email=a@example.com"}, args...)...)
		cmd.Dir = dir
		_ = cmd.Run()
	}
	_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
	_ = os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	run("add", "-A")
	run("commit", "-m", "work on "+name)
}

func newJob() *runner.ClaimedJob {
	return &runner.ClaimedJob{
		ID: jobID, TaskID: "t", RepoOwner: "xdoubleu", RepoName: "tools.xdoubleu.com",
		DefaultBranch: "main", Harness: runner.HarnessClaudeCode, Prompt: "# Task: Fix it",
		Branch: branch, SetupCommands: "make setup", TimeLimitMinutes: 1,
		MCPClientID: "", MCPClientSecret: "",
	}
}

type jobEnv struct {
	api     *fakeAPI
	sandbox *fakeSandbox
	runner  *runner.JobRunner
	dir     string
	remote  string
}

func newEnv(
	t *testing.T,
	sandbox func(context.Context, runner.SandboxSpec) (int, error),
	jobs ...*runner.ClaimedJob,
) *jobEnv {
	t.Helper()
	remote := newRemote(t)
	dir := t.TempDir()
	api := newFakeAPI(jobs...)
	sb := &fakeSandbox{run: sandbox, specs: nil}
	return &jobEnv{
		api: api, sandbox: sb, dir: dir, remote: remote,
		runner: &runner.JobRunner{
			API: api, Token: "frt_x",
			Allowed: func() ([]string, error) {
				return []string{"xdoubleu/tools.xdoubleu.com"}, nil
			},
			Git: &runner.Git{
				Dir: dir, RemoteURL: func(string, string) string { return remote }, Run: nil,
				RunWithEnv: nil,
			},
			Sandbox: sb, Harness: runner.ClaudeCode{}, Activity: nil, DataDir: dir, Secrets: nil,
			GitHub: nil, ServerURL: "",
			CheckInEvery: 10 * time.Millisecond, Minute: time.Second, EventsEvery: 0,
			Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	}
}

func TestAJobRunsInACloneOnItsBranchAndReportsItsOutcome(t *testing.T) {
	env := newEnv(t, writes("Did it.\nPR title: Fix it", 0), newJob())
	activity := runner.NewActivity(awakeStub())
	env.runner.Activity = activity

	ran, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.True(t, ran)

	require.Len(t, env.api.done(), 1)
	got := env.api.done()[0]
	assert.Equal(t, jobID, got.jobID)
	assert.True(t, got.succeeded)
	assert.Equal(t, "Did it.\nPR title: Fix it", got.outcome)
	assert.Equal(t, "sess-9", got.session)
	assert.True(t, activity.Idle(), "the job is over")

	require.Len(t, env.sandbox.specs, 1)
	spec := env.sandbox.specs[0]
	assert.Equal(t, filepath.Join(env.dir, "jobs", jobID, "clone"), spec.CloneDir)
	assert.Equal(t, jobID, spec.Env["FACTORY_JOB"])
	assert.Equal(t, runner.SandboxOutcomePath, spec.Env["FACTORY_OUTCOME_PATH"])
	assert.Equal(t, "make setup", spec.SetupCommands)
	assert.Equal(t, runner.ClaudeCode{}.Command(), spec.Command)
	assert.Contains(
		t,
		spec.Stdin,
		"Commit your work locally on the current branch ("+branch+")",
	)
	assert.True(
		t,
		strings.HasSuffix(spec.Stdin, "# Task: Fix it"),
		"the frozen prompt comes last",
	)

	// The clone is standalone, on the job branch, pointing at the real remote.
	assert.Equal(t, branch, gitIn(t, spec.CloneDir, "branch", "--show-current"))
	assert.Equal(t, env.remote, gitIn(t, spec.CloneDir, "remote", "get-url", "origin"))
	assert.FileExists(t, filepath.Join(spec.CloneDir, "README"))
	assert.NoDirExists(t, filepath.Join(spec.CloneDir, ".git", "worktrees"))
}

func TestACleanExitWithoutAnOutcomeFileIsAFailure(t *testing.T) {
	env := newEnv(t, writes("", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	got := env.api.done()[0]
	assert.False(t, got.succeeded, "nothing for the PR step to use")
	assert.Contains(t, got.outcome, "wrote no outcome file")
}

func TestTheRunnerRefusesWhatItsOwnRulesForbid(t *testing.T) {
	mutate := map[string]func(*runner.ClaimedJob){
		"a repo off the allowlist": func(j *runner.ClaimedJob) { j.RepoOwner = "evil" },
		"a branch outside agent/":  func(j *runner.ClaimedJob) { j.Branch = "main" },
		"an option as the branch": func(j *runner.ClaimedJob) {
			j.Branch = "--upload-pack=x"
		},
		"an option as the base":      func(j *runner.ClaimedJob) { j.DefaultBranch = "-x" },
		"a job id that names a path": func(j *runner.ClaimedJob) { j.ID = "../../etc" },
		"a harness it can't run": func(j *runner.ClaimedJob) {
			j.Harness = "HARNESS_OPENCODE"
		},
	}
	for name, change := range mutate {
		job := newJob()
		change(job)
		env := newEnv(t, writes("x", 0), job)

		ran, err := env.runner.Next(context.Background())
		require.NoError(t, err, name)
		assert.True(t, ran, name)
		require.Len(t, env.api.done(), 1, name)
		assert.False(t, env.api.done()[0].succeeded, name)
		assert.Contains(t, env.api.done()[0].outcome, "Refused by the runner", name)
		assert.Empty(t, env.sandbox.specs, name+": nothing started")
		assert.NoDirExists(t, filepath.Join(env.dir, "jobs"), name+": nothing was cloned")
	}
}

func TestAnAllowlistThatCannotBeReadRefusesTheJob(t *testing.T) {
	env := newEnv(t, writes("x", 0), newJob())
	env.runner.Allowed = func() ([]string, error) {
		return nil, errors.New("config broken")
	}
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "config broken")
	assert.Empty(t, env.sandbox.specs)
}

func TestFailuresAreReportedWithTheReasonAndWhatTheAgentWrote(t *testing.T) {
	for name, tc := range map[string]struct {
		run  func(context.Context, runner.SandboxSpec) (int, error)
		want string
	}{
		"a failing exit": {writes("Half done.", 3), "exited with status 3"},
		"a sandbox that can't start": {
			func(context.Context, runner.SandboxSpec) (int, error) {
				return 0, errors.New("no vm")
			}, "could not run the job: no vm",
		},
	} {
		env := newEnv(t, tc.run, newJob())
		_, err := env.runner.Next(context.Background())
		require.NoError(t, err, name)
		got := env.api.done()[0]
		assert.False(t, got.succeeded, name)
		assert.Contains(t, got.outcome, tc.want, name)
	}

	env := newEnv(t, writes("Half done.", 3), newJob())
	_, _ = env.runner.Next(context.Background())
	assert.Contains(t, env.api.done()[0].outcome, "Half done.")
}

func TestAJobPastItsTimeLimitIsStopped(t *testing.T) {
	env := newEnv(t, func(ctx context.Context, _ runner.SandboxSpec) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}, newJob())
	start := time.Now()
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "Timed out after 1 minutes")
}

func TestCheckInsCarryTheSessionAndAStopEndsTheJob(t *testing.T) {
	env := newEnv(t, func(ctx context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, `{"session_id":"sess-1"}`+"\n")
		<-ctx.Done()
		return 0, ctx.Err()
	}, newJob())
	env.api.stop = true

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "Stopped")
	assert.Equal(t, "sess-1", got.session)
	env.api.mu.Lock()
	defer env.api.mu.Unlock()
	assert.Contains(t, env.api.checkIns, "sess-1", "a check-in sent the session id")
}

func TestShuttingDownStillReportsTheJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	env := newEnv(t, func(c context.Context, _ runner.SandboxSpec) (int, error) {
		cancel()
		<-c.Done()
		return 0, c.Err()
	}, newJob())
	_, err := env.runner.Next(ctx)
	require.NoError(t, err)
	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "shut down")
}

func TestACloneThatCannotBeMadeFailsTheJob(t *testing.T) {
	env := newEnv(t, writes("x", 0), newJob())
	env.runner.Git.RemoteURL = func(string, string) string {
		return filepath.Join(env.dir, "nope.git")
	}
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	got := env.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "Could not prepare the clone")
	assert.Empty(t, env.sandbox.specs)
}

func TestTheMirrorIsUpdatedForTheNextJob(t *testing.T) {
	first := newJob()
	second := newJob()
	second.ID = "99999999-2222-3333-4444-555555555555"
	second.Branch = "agent/33333333-3333-4444-5555-666666666666-next"
	env := newEnv(t, writes("ok", 0), first, second)

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	// A commit lands upstream between the jobs.
	work := t.TempDir()
	gitIn(t, work, "clone", env.remote, "w")
	w := filepath.Join(work, "w")
	require.NoError(t, os.WriteFile(filepath.Join(w, "NEW"), []byte("x"), 0o644))
	gitIn(t, w, "add", ".")
	gitIn(t, w, "commit", "-m", "second")
	gitIn(t, w, "push", "origin", "main")

	_, err = env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(env.dir, "jobs", second.ID, "clone", "NEW"))
}

func TestACloneDoesNotShareObjectsWithTheMirror(t *testing.T) {
	env := newEnv(t, writes("ok", 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	// A hostile write through the mount must not reach the mirror: no hard links,
	// and no alternates file pointing at it.
	clone := env.sandbox.specs[0].CloneDir
	assert.NoFileExists(t, filepath.Join(clone, ".git", "objects", "info", "alternates"))
	objects := gitIn(t, clone, "rev-parse", "HEAD")
	assert.NotEmpty(t, objects)
}

func TestLoopRunsJobsUntilItsContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	second := newJob()
	second.ID = "99999999-2222-3333-4444-555555555555"
	second.Branch = "agent/33333333-3333-4444-5555-666666666666-next"
	env := newEnv(t, writes("ok", 0), newJob(), second)

	go func() {
		for range 2 {
			<-env.api.completes
		}
		cancel()
	}()
	require.NoError(t, env.runner.LoopForTest(ctx,
		func(context.Context, time.Duration) {}, ticking()))
	assert.Len(t, env.api.done(), 2)
}

func TestLoopBacksOffOnErrorsAndEndsWhenRevoked(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	env.api.claimErr = errors.New("offline")
	ctx, cancel := context.WithCancel(context.Background())
	var slept []time.Duration
	calls := 0
	err := env.runner.LoopForTest(ctx, func(_ context.Context, d time.Duration) {
		slept = append(slept, d)
		calls++
		if calls == 3 {
			env.api.mu.Lock()
			env.api.claimErr = &runner.Error{Code: "unauthenticated", Message: "revoked"}
			env.api.mu.Unlock()
		}
	}, ticking())
	cancel()
	require.ErrorIs(t, err, runner.ErrRevoked)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, slept)
}

func TestLoopStopsQuietlyWhenCancelledDuringAClaim(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, env.runner.LoopForTest(ctx,
		func(context.Context, time.Duration) {}, ticking()))

	ctx2, cancel2 := context.WithCancel(context.Background())
	env.api.claimErr = errors.New("context canceled")
	err := env.runner.LoopForTest(ctx2, func(context.Context, time.Duration) {
		cancel2()
	}, ticking())
	require.NoError(t, err)
}

func TestLoopWaitsOutAFastEmptyClaim(t *testing.T) {
	env := newEnv(t, writes("ok", 0))
	ctx, cancel := context.WithCancel(context.Background())
	var slept []time.Duration
	_ = env.runner.LoopForTest(ctx, func(_ context.Context, d time.Duration) {
		slept = append(slept, d)
		cancel()
	}, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	assert.Equal(
		t,
		[]time.Duration{time.Second},
		slept,
		"an instant empty answer isn't repeated at once",
	)
}

func TestTheOutcomeIsOnlyReadFromARegularFileUpToItsCap(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(secret, []byte("host secret"), 0o600))

	for name, plant := range map[string]func(path string){
		"a symlink to a host file": func(p string) {
			require.NoError(t, os.Symlink(secret, p))
		},
		"a fifo": func(p string) { require.NoError(t, syscall.Mkfifo(p, 0o600)) },
	} {
		env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
			plant(filepath.Join(spec.OutcomeDir, "outcome.md"))
			return 0, nil
		}, newJob())
		_, err := env.runner.Next(context.Background())
		require.NoError(t, err, name)
		got := env.api.done()[0]
		assert.False(t, got.succeeded, name)
		assert.NotContains(t, got.outcome, "host secret", name)
	}

	env := newEnv(t, writes(strings.Repeat("x", 50000), 0), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Len(t, env.api.done()[0].outcome, 9000, "capped")
}

func TestATimeoutNamesTheLimitThatApplied(t *testing.T) {
	hang := func(ctx context.Context, _ runner.SandboxSpec) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	job := newJob()
	job.TimeLimitMinutes = 2
	env := newEnv(t, hang, job)
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, env.api.done()[0].outcome, "Timed out after 2 minutes")
}

func TestTheServerCannotStretchTheTimeLimit(t *testing.T) {
	job := newJob()
	job.TimeLimitMinutes = 1 << 40
	var deadline time.Time
	env := newEnv(t, func(ctx context.Context, _ runner.SandboxSpec) (int, error) {
		deadline, _ = ctx.Deadline()
		return 0, nil
	}, job)
	env.runner.Minute = time.Minute
	start := time.Now()
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.WithinDuration(t, start.Add(24*time.Hour), deadline, time.Minute)
}

func TestAJobClaimedAgainStartsFromACleanDirectory(t *testing.T) {
	env := newEnv(t, writes("ok", 0), newJob(), newJob())
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	stale := filepath.Join(env.dir, "jobs", jobID, "clone", "STALE")
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o644))

	_, err = env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.True(
		t,
		env.api.done()[1].succeeded,
		"the second clone did not collide with the first",
	)
	assert.NoFileExists(t, stale)
}

func TestABrokenMirrorIsReplaced(t *testing.T) {
	env := newEnv(t, writes("ok", 0), newJob())
	// An interrupted first clone leaves a directory that is no repository.
	broken := filepath.Join(env.dir, "mirrors", "xdoubleu", "tools.xdoubleu.com.git")
	require.NoError(t, os.MkdirAll(broken, 0o755))

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.True(t, env.api.done()[0].succeeded)
}

func TestAJobWithoutARepoLimitGetsTheDefault(t *testing.T) {
	job := newJob()
	job.TimeLimitMinutes = 0
	var deadline time.Time
	env := newEnv(t, func(ctx context.Context, _ runner.SandboxSpec) (int, error) {
		deadline, _ = ctx.Deadline()
		return 0, nil
	}, job)
	env.runner.Minute = time.Minute
	start := time.Now()
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.WithinDuration(t, start.Add(runner.DefaultTimeLimit), deadline, time.Minute)
}
