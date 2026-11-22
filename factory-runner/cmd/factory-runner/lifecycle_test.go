package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// The lifecycle tests run the real serve loop, daemon, claim loop, job loop and
// update drain against a fake board, sandbox, GitHub and updater, and check that
// the runner always gets back to claiming. Each stall the runner has had is a
// scenario here.

const lifePass = "Did it.\n\nChecks: lint=pass, test=pass, docs=pass"

func life(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t",
		"-c", "user.email=t@example.com", "-c", "init.defaultBranch=main",
		"-c", "protocol.file.allow=always"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// lifeBoard is the server: queued jobs, a heartbeat and the claim hold.
type lifeBoard struct {
	mu        sync.Mutex
	queue     []*runner.ClaimedJob
	running   int
	claims    int
	beats     int
	beatFails int
	done      map[string]runner.JobEnd
}

func (b *lifeBoard) Heartbeat(ctx context.Context, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.beatFails > 0 {
		b.beatFails--
		return errors.New("502 bad gateway")
	}
	b.beats++
	return ctx.Err()
}

func (b *lifeBoard) Queued() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.queue)
}

func (b *lifeBoard) Pair(context.Context, string) (string, string, error) {
	return "frt_x", "Mac", nil
}

func (b *lifeBoard) enqueue(jobs ...*runner.ClaimedJob) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queue = append(b.queue, jobs...)
}

func (b *lifeBoard) ClaimJob(
	ctx context.Context, _ string, maxJobs int, _, _ string,
) (*runner.ClaimedJob, error) {
	b.mu.Lock()
	if b.running < maxJobs && len(b.queue) > 0 {
		job := b.queue[0]
		b.queue = b.queue[1:]
		b.running++
		b.claims++
		b.mu.Unlock()
		return job, nil
	}
	b.mu.Unlock()
	select {
	case <-ctx.Done():
	case <-time.After(20 * time.Millisecond):
	}
	return nil, nil //nolint:nilnil // none within the hold
}

func (b *lifeBoard) CompleteJob(
	_ context.Context, _, id string, end runner.JobEnd,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running--
	b.done[id] = end
	return nil
}

func (b *lifeBoard) completed(id string) (runner.JobEnd, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	end, ok := b.done[id]
	return end, ok
}

func (*lifeBoard) CheckIn(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func (*lifeBoard) AppendJobEvents(
	context.Context, string, string, []runner.EventLine,
) error {
	return nil
}

func (*lifeBoard) GetPushToken(context.Context, string, string) (string, error) {
	return "ghs_pushtoken_123456", nil
}

func (*lifeBoard) ListFinishedTasks(
	context.Context, string, []string,
) ([]string, error) {
	return nil, nil
}

func (*lifeBoard) AddJobComment(context.Context, string, string, string) error {
	return nil
}

// lifeGitHub merges every pull request on the first read.
type lifeGitHub struct{}

func (lifeGitHub) OpenPullRequest(
	context.Context, string, runner.PullRequest,
) (string, error) {
	return "https://github.com/xdoubleu/tools.xdoubleu.com/pull/7", nil
}

func (lifeGitHub) PullRequestStatus(
	context.Context, string, runner.PullRef,
) (runner.PullStatus, error) {
	return runner.PullStatus{
		State: runner.PullMerged, HeadSHA: "", CI: "", Failed: nil, Conflict: false,
		NoChecks: false,
	}, nil
}

// lifeSandbox runs the host's base sync for real and every agent run as agent.
type lifeSandbox struct {
	agent func(ctx context.Context, spec runner.SandboxSpec) (int, error)
	mu    sync.Mutex
	clone []string
}

func (s *lifeSandbox) Run(ctx context.Context, spec runner.SandboxSpec) (int, error) {
	if len(spec.Command) == 5 && spec.Command[3] == "sync" {
		cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
		cmd.Dir = spec.CloneDir
		if err := cmd.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return ee.ExitCode(), nil
			}
			return 1, err
		}
		return 0, nil
	}
	s.mu.Lock()
	s.clone = append(s.clone, spec.CloneDir)
	s.mu.Unlock()
	return s.agent(ctx, spec)
}

// lifeUpdater has a newer release once available is set; like the real one it
// waits for idle and then replaces the binary.
type lifeUpdater struct {
	available atomic.Bool
	installed atomic.Bool
}

func (u *lifeUpdater) Update(_ context.Context, idle func() bool) (bool, error) {
	if !u.available.Load() {
		return false, nil
	}
	if !idle() {
		return false, runner.ErrUpdatePending
	}
	u.installed.Store(true)
	u.available.Store(false)
	return true, nil
}

type lifeEnv struct {
	t       *testing.T
	remote  string
	board   *lifeBoard
	sandbox *lifeSandbox
	updater *lifeUpdater
	// imageErr, while set, fails building the jobs, as a failed image pull does.
	imageErr atomic.Pointer[string]
	maxJobs  int
}

func newLife(t *testing.T) *lifeEnv {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	require.NoError(t, os.MkdirAll(work, 0o755))
	life(t, work, "init")
	require.NoError(t, os.WriteFile(filepath.Join(work, "README"), []byte("hi"), 0o644))
	life(t, work, "add", ".")
	life(t, work, "commit", "-m", "first")
	bare := filepath.Join(root, "remote.git")
	life(t, root, "clone", "--bare", work, bare)
	e := &lifeEnv{
		t: t, remote: bare, updater: &lifeUpdater{
			available: atomic.Bool{}, installed: atomic.Bool{},
		}, maxJobs: 1, sandbox: nil, imageErr: atomic.Pointer[string]{},
		board: &lifeBoard{
			mu: sync.Mutex{}, queue: nil, running: 0, claims: 0, beats: 0,
			beatFails: 0, done: map[string]runner.JobEnd{},
		},
	}
	e.sandbox = &lifeSandbox{agent: e.agentWrites, mu: sync.Mutex{}, clone: nil}
	oldSleep := jobRetrySleep
	jobRetrySleep = func(ctx context.Context, _ time.Duration) {
		select {
		case <-ctx.Done():
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Cleanup(func() { jobRetrySleep = oldSleep })
	return e
}

// agentWrites commits a file and writes a passing outcome.
func (e *lifeEnv) agentWrites(_ context.Context, spec runner.SandboxSpec) (int, error) {
	name := fmt.Sprintf("agent-%d.txt", time.Now().UnixNano())
	_ = os.WriteFile(filepath.Join(spec.CloneDir, name), []byte("work"), 0o644)
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", "work " + name}} {
		cmd := exec.Command("git", append([]string{"-c", "user.name=a",
			"-c", "user.email=a@example.com"}, args...)...)
		cmd.Dir = spec.CloneDir
		_ = cmd.Run()
	}
	_ = os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"), []byte(lifePass), 0o644)
	return 0, nil
}

func lifeJob(n int) *runner.ClaimedJob {
	return &runner.ClaimedJob{
		ID:        fmt.Sprintf("11111111-2222-3333-4444-%012d", n),
		TaskID:    fmt.Sprintf("T-%d", n),
		RepoOwner: "xdoubleu", RepoName: "tools.xdoubleu.com", DefaultBranch: "main",
		Prompt: "# Task: Fix it", Branch: fmt.Sprintf("agent/%d-work", n),
		TimeLimitMinutes: 1, MCPClientID: "", MCPClientSecret: "",
		ResumeSessionID: "", Routine: "", ChatID: "",
	}
}

// generation is one run of the process: what launchd starts again after a restart.
type generation struct {
	app      *app
	done     chan error
	stopped  chan struct{}
	cancel   context.CancelFunc
	runnerOf atomic.Pointer[runner.JobRunner]
}

func (e *lifeEnv) start() *generation {
	e.t.Helper()
	a := newEnv(e.t).app
	a.activity = runner.NewActivity(noAwake{})
	a.control = runner.NewControl(runner.PausePath(a.home), "test", a.activity)
	a.control.SetModeSource(a.mode)
	a.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg, path, err := a.loadConfig()
	require.NoError(e.t, err)
	cfg.Mode = runner.ModeClaude
	require.NoError(e.t, cfg.SetMaxJobs(e.maxJobs))
	require.NoError(e.t, cfg.Save(path))
	g := &generation{
		app: a, done: make(chan error, 1), stopped: make(chan struct{}),
		cancel: nil, runnerOf: atomic.Pointer[runner.JobRunner]{},
	}
	a.newBeat = func(string) pairHeartbeater { return e.board }
	a.jobs = func(_ context.Context, _ runner.Config, _ string) (jobLoop, error) {
		if msg := e.imageErr.Load(); msg != nil {
			return nil, errors.New(*msg)
		}
		dir := e.t.TempDir()
		jobs := &runner.JobRunner{
			OpenChat:  nil,
			NoCIGrace: 0,
			API:       e.board, Token: "frt_x", Paused: a.holdClaims, MaxJobs: a.maxJobs,
			Mode: a.mode, Harness: runner.ClaudeCode{},
			Allowed: func() ([]string, error) {
				return []string{"xdoubleu/tools.xdoubleu.com"}, nil
			},
			Git: &runner.Git{
				Dir: dir, RemoteURL: func(string, string) string { return e.remote },
				Run: nil, RunWithEnv: nil,
			},
			ClaudeToken: nil, OpenRouterKey: nil, AuthorEmail: "",
			MCP: nil, Proxy: nil, Sessions: nil, OnLimit: nil, DiskFree: nil, Secrets: nil,
			EventsEvery: 0, RunningPollEvery: 0, Sleep: nil, Now: nil,
			Sandbox: e.sandbox, Activity: a.activity, DataDir: dir,
			GitHub: lifeGitHub{}, ServerURL: "https://tools.example.com/",
			PublishRetry: time.Millisecond, CheckInEvery: 10 * time.Millisecond,
			Minute: time.Second, PollEvery: time.Millisecond, MaxWatch: 5 * time.Second,
			Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
		a.control.SetClaimSources(runner.ClaimSources{
			Max: jobs.MaxJobsNow, Jobs: jobs.RunningJobs, LimitUntil: jobs.LimitUntil,
		})
		g.runnerOf.Store(jobs)
		return jobs, nil
	}
	a.daemon = func(
		ctx context.Context, hb runner.Heartbeater, token string, opts runner.Options,
	) error {
		opts.UpdateEvery = 50 * time.Millisecond
		opts.Sleep = func(ctx context.Context, _ time.Duration) {
			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Millisecond):
			}
		}
		quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
		return runner.Run(ctx, hb, token, quiet, opts)
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	cfg.Server = "https://tools.example.com/api"
	beat := func(ctx context.Context) error {
		opts := runner.Options{
			Updater: e.updater, Activity: a.activity, Control: a.control,
			UpdateEvery: 0, Sleep: nil,
		}
		return a.daemon(ctx, e.board, "frt_x", opts)
	}
	go func() {
		g.done <- a.serve(ctx, cfg, "frt_x", beat)
		close(g.stopped)
	}()
	e.t.Cleanup(func() {
		cancel()
		select {
		case <-g.stopped:
		case <-time.After(10 * time.Second):
		}
	})
	return g
}

type noAwake struct{}

func (noAwake) Start() (func(), error) { return func() {}, nil }

func (e *lifeEnv) waitDone(ids ...string) {
	e.t.Helper()
	require.Eventually(e.t, func() bool {
		for _, id := range ids {
			if _, ok := e.board.completed(id); !ok {
				return false
			}
		}
		return true
	}, 15*time.Second, 10*time.Millisecond, "jobs %v did not finish", ids)
}

func (e *lifeEnv) succeeded(id string) bool {
	end, _ := e.board.completed(id)
	return end.Succeeded
}

// provesClaiming queues one more job and waits for it: the runner is claiming.
func (e *lifeEnv) provesClaiming(n int) {
	e.t.Helper()
	job := lifeJob(n)
	e.board.enqueue(job)
	e.waitDone(job.ID)
	assert.True(e.t, e.succeeded(job.ID))
}

func TestAnUpdateArrivingMidJobDrainsRestartsAndClaimsAgain(t *testing.T) {
	e := newLife(t)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	e.sandbox.agent = func(ctx context.Context, spec runner.SandboxSpec) (int, error) {
		once.Do(func() { close(started) })
		<-release
		return e.agentWrites(ctx, spec)
	}
	first, second := lifeJob(1), lifeJob(2)
	e.board.enqueue(first)
	g := e.start()
	<-started

	e.updater.available.Store(true)
	g.app.control.RequestCheck()
	require.Eventually(t, g.app.control.Draining, 5*time.Second, 5*time.Millisecond)
	e.board.enqueue(second)
	time.Sleep(300 * time.Millisecond)
	e.board.mu.Lock()
	claims := e.board.claims
	e.board.mu.Unlock()
	assert.Equal(t, 1, claims, "no claim while draining")
	assert.Contains(t, g.app.control.ClaimState().Reason, "draining")

	close(release)
	e.waitDone(first.ID)
	select {
	case err := <-g.done:
		require.NoError(t, err, "the runner restarts once the job ended")
	case <-time.After(10 * time.Second):
		t.Fatal("the runner never restarted after the update")
	}
	assert.True(t, e.updater.installed.Load())

	e.sandbox.agent = e.agentWrites
	e.start()
	e.waitDone(second.ID)
	assert.True(t, e.succeeded(second.ID))
}

func TestAJobWhosePullRequestIsMergedAtOnceFreesItsSlot(t *testing.T) {
	e := newLife(t)
	first := lifeJob(1)
	e.board.enqueue(first)
	g := e.start()
	e.waitDone(first.ID)
	end, _ := e.board.completed(first.ID)
	assert.NotEmpty(t, end.PRURL)
	assert.True(t, end.Succeeded)

	e.provesClaiming(2)
	assert.Eventually(t, g.app.activity.Idle, 5*time.Second, 5*time.Millisecond)
	assert.Empty(t, g.runnerOf.Load().RunningJobs())
}

func TestJobsComeBackAfterTheImagePullFailsThenSucceeds(t *testing.T) {
	e := newLife(t)
	msg := "pull image: no space left on device"
	e.imageErr.Store(&msg)
	job := lifeJob(1)
	e.board.enqueue(job)
	g := e.start()

	require.Eventually(t, func() bool {
		return strings.Contains(g.app.control.ClaimState().Reason, "no space left")
	}, 5*time.Second, 5*time.Millisecond, "the status names the image error")
	assert.False(t, g.app.control.ClaimState().Claiming)
	e.board.mu.Lock()
	assert.Zero(t, e.board.claims)
	e.board.mu.Unlock()

	e.imageErr.Store(nil)
	g.app.control.KickJobs()
	e.waitDone(job.ID)
	assert.True(t, e.succeeded(job.ID))
	assert.True(t, g.app.control.ClaimState().Claiming)
}

func TestAHeartbeatOutageDoesNotStopClaimsAndTheBeatComesBack(t *testing.T) {
	e := newLife(t)
	e.board.beatFails = 5
	first := lifeJob(1)
	e.board.enqueue(first)
	g := e.start()
	e.waitDone(first.ID)

	require.Eventually(t, func() bool {
		e.board.mu.Lock()
		defer e.board.mu.Unlock()
		return e.board.beats > 0
	}, 5*time.Second, 5*time.Millisecond, "the heartbeat resumes")
	assert.Eventually(t, func() bool {
		return !g.app.control.Snapshot().LastHeartbeat.IsZero()
	}, 5*time.Second, 5*time.Millisecond)
	e.provesClaiming(2)
}

func TestTwoSlotsTakeTheFirstTwoOfThreeTasksAndThenTheThird(t *testing.T) {
	e := newLife(t)
	e.maxJobs = 2
	var running, peak, arrived atomic.Int32
	both := make(chan struct{})
	e.sandbox.agent = func(ctx context.Context, spec runner.SandboxSpec) (int, error) {
		n := running.Add(1)
		defer running.Add(-1)
		for p := peak.Load(); n > p; p = peak.Load() {
			if peak.CompareAndSwap(p, n) {
				break
			}
		}
		// The first two jobs wait for each other, however slowly a loaded
		// machine gets the second one going; the third runs alone.
		if arrived.Add(1) == 2 {
			close(both)
		}
		if arrived.Load() <= 2 {
			select {
			case <-both:
			case <-time.After(10 * time.Second):
				return 1, errors.New("the other slot never started")
			}
		}
		return e.agentWrites(ctx, spec)
	}
	jobs := []*runner.ClaimedJob{lifeJob(1), lifeJob(2), lifeJob(3)}
	e.board.enqueue(jobs...)
	e.start()

	e.waitDone(jobs[0].ID, jobs[1].ID, jobs[2].ID)
	assert.Equal(t, int32(2), peak.Load(), "both slots are used, never more")
	for _, j := range jobs {
		assert.True(t, e.succeeded(j.ID))
	}
}

func TestAResumedBranchOnAnOldBaseIsBroughtUpToDateAndFinishes(t *testing.T) {
	e := newLife(t)
	first := lifeJob(1)
	e.board.enqueue(first)
	e.start()
	e.waitDone(first.ID)

	// main moves while the task waits; the task is re-run on its old branch.
	work := t.TempDir()
	life(t, work, "clone", e.remote, ".")
	file := filepath.Join(work, "main-only.txt")
	require.NoError(t, os.WriteFile(file, []byte("new"), 0o644))
	life(t, work, "add", ".")
	life(t, work, "commit", "-m", "main moves")
	life(t, work, "push", "origin", "HEAD")

	rerun := lifeJob(1)
	rerun.ID = "11111111-2222-3333-4444-000000000099"
	e.board.enqueue(rerun)
	e.waitDone(rerun.ID)
	assert.True(t, e.succeeded(rerun.ID))
	e.sandbox.mu.Lock()
	clone := e.sandbox.clone[len(e.sandbox.clone)-1]
	e.sandbox.mu.Unlock()
	assert.FileExists(t, filepath.Join(clone, "main-only.txt"), "the base was merged in")
	e.provesClaiming(2)
}
