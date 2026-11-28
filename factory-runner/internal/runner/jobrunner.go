package runner

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"tools.xdoubleu.com/wardcase/session"
)

const (
	// DefaultTimeLimit applies when a repo sets none.
	DefaultTimeLimit = 60 * time.Minute
	// CheckInEvery is how often a running job reports alive; the server calls a
	// job lost after five minutes of silence.
	CheckInEvery = 30 * time.Second

	completeTimeout = 30 * time.Second
	maxOutcomeBytes = 9000
	maxTapLine      = 1 << 20
	jobDirPerm      = 0o700
	// outcomeDirPerm lets the sandbox's non-root user write the outcome.
	outcomeDirPerm = 0o777
	outcomeFile    = "outcome.md"
	// maxTimeLimitMinutes caps a job at a day, whatever the server says.
	maxTimeLimitMinutes = 24 * 60
)

var jobIDPattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

// taskIDPattern is a task id: T-<number>, or the UUID of a task from before
// numbers. It names a session directory.
var taskIDPattern = regexp.MustCompile(
	`^(T-[0-9]{1,18}|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`,
)

// sessionIDPattern is what a harness session id may look like: it goes into the
// harness's argv, so nothing that could be read as a flag or a path.
var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)

// retroPrompt ends a job that needed retries; its arguments are the phase times
// and the cause of each retry.
const retroPrompt = `The job is over. Do not touch the repository, commit or ` +
	`push, and write no outcome file. Look back on it: %s.

Why it needed more than one attempt:
%s

List repeatable friction (a missing tool, a gate you could not run, a CI ` +
	`failure the local gates missed, slow steps). For each, search ` +
	`factory_list_tasks (open columns, label factory-retro and again label ` +
	`factory-sandbox) and comment on a task with the same cause using ` +
	`factory_comment_task, also when the job's Blocked line already names ` +
	`one. Only when none matches, file one with factory_create_task, label ` +
	`factory-retro (never factory-sandbox), naming the cause. File nothing ` +
	`when nothing is repeatable. Reply in one line.`

// continuePrompt is added when a job without a session starts on a branch that
// already holds earlier work (a task re-run from Ready). %s is the default branch.
const continuePrompt = `You are continuing earlier work on this branch: its ` +
	`commits are already here. Merge origin/%s (the host fetched it into ` +
	`this clone) if it moved, then add commits on top. Never rebase or rewrite ` +
	`existing commits: the push is a fast-forward.`

// preamble is the runner's own, repo-agnostic part of every prompt; the server
// can't change it. %s is the job's branch.
const preamble = `You are running unattended in a sandbox with no network ` +
	`except the model. ` +
	`There is no GitHub: do not try to push or open a pull request. Commit your ` +
	`work locally on the current branch (%s). When you are done, write a short ` +
	`outcome to the file named by $FACTORY_OUTCOME_PATH: its first line is the ` +
	`pull request title, then what you did and what is left. Before you commit, ` +
	`run the repository's lint, test and documentation-budget gates (see its ` +
	`AGENTS.md). Create new migrations with ` +
	"`make migration/new app=<app> name=<name>`" + ` (never copy a 000NN ` +
	`name) and run ` + "`make lint/sql lint/migrations`" + ` before the gate. ` +
	`End the outcome with one line listing each gate, for example ` +
	`"Checks: lint=pass, test=pass, docs=pass"; a gate is pass, fail or ` +
	`not run. Mark a gate not run only when its target fails to run here, ` +
	`never because you judged it unnecessary or slow. A gate its target can ` +
	`run in the sandbox counts as run, even ` +
	`when your change has no code diff. That Checks: line must be the final ` +
	`line of the file and start ` +
	`with "Checks:": put no text after it and no prefix before it, and when ` +
	`you rewrite the outcome keep exactly one. Where the repository gates the ` +
	`coverage of changed lines (see its AGENTS.md), run that check locally ` +
	`too, not only the tests: in each changed Go module run ` +
	"`make test/cov/diff`" + ` (CI's ` + "`scripts/diff_coverage.sh`" + `, ` +
	`80 percent of changed lines) and add tests until it passes. No pull request is ` +
	`opened unless every gate ` +
	`passed or was not ` +
	`run because CI checks it. Code the sandbox cannot build or test, such as ` +
	`darwin or cgo code, is not a blocker: CI's macOS jobs check it, so mark ` +
	`that gate "not run (CI checks it)", for example "build-mac=not run (CI ` +
	`checks it)". The same goes for the docker-only gates ` +
	"`grafana/verify` and `lint/infra`" + `: the sandbox has no docker, so mark ` +
	`them "not run (CI checks it)", never a bare "not run". ` +
	"`make factory/finish`" + ` runs every gate for the changed modules offline and ` +
	`prints the Checks: line. ` +
	`Your work continues until its pull request merges: the ` +
	`runner opens the pull request, and when a gate, the push or CI fails it ` +
	`resumes this session with what failed. Fix it, commit, and write the ` +
	`outcome again. If the task depends on other tasks that are not done yet, ` +
	`add one line "Waiting-on: T-6, T-8" naming them and nothing else: the ` +
	`task goes back to Backlog and no sandbox task is filed. A blocker that ` +
	`is not the sandbox (an upstream release not yet tagged, work another ` +
	`task tracks, in any repository) is that: before reporting "Blocked:", ` +
	`look for the task that tracks it with factory_list_tasks, without a ` +
	`repo_id so every repository is searched, and when one exists name it ` +
	`in a Waiting-on line, never as Blocked and never as a ` +
	`factory-sandbox task. A line ` +
	`"Blocked:" in the outcome stops that loop for good ` +
	`unless a pull request opened, ` +
	`so use it only for a sandbox or tool gap, never for a dependency on ` +
	`other tasks: if the sandbox stops ` +
	`you doing or checking the work (a missing module cache, blocked network, ` +
	`absent tool), search the board with factory_list_tasks for the label ` +
	`factory-sandbox among open and In review tasks, matching on the missing ` +
	`tool or cache name (node_modules, go mod, ruby, postgres), not the whole ` +
	`title, and add a comment with factory_comment_task to any match, ` +
	`otherwise file one with factory_create_task ` +
	`titled "Sandbox: <short problem>". Then carry on as far as you can, and ` +
	`add one line "Blocked: <the problem>" to the outcome. ` +
	`If the gap is a missing tool, also fix it: on a separate local branch ` +
	`agent/sandbox-<slug> created with "git switch -c agent/sandbox-<slug> ` +
	`origin/<the repository's default branch>" (never off your job branch, which ` +
	`would carry its files), commit a change ` +
	`to the repository's flake.nix and flake.lock only (create flake.nix with a ` +
	`devShell holding the tool when the repository has none; the lock is the ` +
	`pin), and say in the "Blocked:" line that the repository's flake lacks the ` +
	`tool. ` +
	`Switch back to your job branch, and add the ` +
	`line "Sandbox-fix: agent/sandbox-<slug>" to the outcome. The runner ` +
	`opens that as its own pull request and refuses any other change; a ` +
	`refused fix comes back to you with the reason. When the fix is the whole ` +
	`task, the job branch needs no commit of its own. ` +
	`For a task labelled feature, also run the feature-review gate ` +
	`(docs/convention-feature-review-policy.md): diff-scoped mutation tests ` +
	`and a code review with findings applied, then add the line ` +
	`"Feature gate: mutation=pass, review=pass". If the sandbox cannot run ` +
	`it, write "Feature gate: mutation=skipped, review=skipped (<why>)"; CI ` +
	`then blocks the merge for the owner. ` +
	`If the problem the task reports no longer reproduces (re-run the ` +
	`reported check first, whatever the task body says), there is nothing to ` +
	`commit: comment the evidence with factory_comment_task, move the task to ` +
	`done with factory_move_task, and end the outcome with one line ` +
	`"Already-fixed: <evidence>" instead of a Checks: line; no gate runs ` +
	`and no pull request opens. ` +
	`A task labelled spike (or one whose deliverable is a board comment) ` +
	`still needs a commit, or the runner refuses the branch: record the ` +
	`result in a docs ADR (see AGENTS.md) and commit it, and post the ` +
	`comment as well. ` +
	`When the only thing that blocked a task is fixed (your sandbox fix ` +
	`shipped, or the task it waited on landed), move it to Ready yourself ` +
	`with factory_move_task and say why; Needs you is only for decisions a ` +
	`person must make. A task parked while a new job image ships, or while its ` +
	`sandbox-fix PR merges, resumes on its own. ` +
	`Before coding, check the task fits one job: a migration plus new RPCs ` +
	`plus a new UI, or several apps, does not (a 60-minute timeout with ` +
	`nothing pushed is the cost). If it is clearly larger, file slices that ` +
	`each fit one job with factory_create_task (Backlog, label ` +
	"`epic:<slug>`" + `), comment their ids on the task with ` +
	`factory_comment_task, and end the outcome with "Waiting-on: <slice ids>" ` +
	`instead of attempting a partial change. ` +
	`Nobody can answer questions while you run.`

// JobRunner claims jobs from the server and runs each one in a sandbox.
type JobRunner struct {
	API     JobAPI
	Token   string
	Allowed func() ([]string, error)
	Git     *Git
	Sandbox Sandbox
	Harness Harness
	// Mode reports this Mac's config mode (ModeClaude or ModeOff); nil means
	// ModeClaude. The job's own harness and model are ignored.
	Mode func() string
	// Paused, when it reports true, stops the loop claiming new jobs.
	Paused func() bool
	// MaxJobs is how many jobs run at once, read at each claim; nil or below 1
	// means one.
	MaxJobs func() int
	// ClaudeToken returns the Claude setup-token a Claude Code job runs on; it is
	// put in the VM's environment (adr-0043's conservative route) and kept out of
	// commits and the outcome like any secret. Nil sets none.
	ClaudeToken func() (string, error)
	// OpenRouterKey returns the key an OpenCode job's model calls run on; it is
	// only checked before a claim, the egress proxy adds it. Nil skips the check.
	OpenRouterKey func() (string, error)
	// AuthorEmail is the address commits carry; the name is the job's board account.
	AuthorEmail string
	// MCP returns the device's MCP servers a job gets (the ones the owner marked
	// jobs = true and signed in) and the names of marked ones that are not signed
	// in; nil gives a job none.
	MCP func() (servers []MCPServer, missing []string, err error)
	// Proxy builds a job's egress proxy, with the MCP servers the job gets; nil
	// gives the job none.
	Proxy func(job *ClaimedJob, servers []MCPServer) (ProxyListener, error)
	// OpenChat opens a chat's harness session; nil means wardcase's, in a VM from
	// Sandbox. Tests replace it.
	OpenChat ChatOpener
	// Activity, when set, holds the Mac awake and blocks self-update while a job runs.
	Activity *Activity
	// Sessions runs a Claude job's attempts through wardcase's session API; nil
	// runs them in the job's own VM session. OnLimit says what a usage limit does:
	// OnLimitFallback or OnLimitWait, and the OpenRouter model id it falls back to;
	// nil waits.
	Sessions func(session.Options) SessionAPI
	OnLimit  func() (policy, model string)
	// DataDir holds <DataDir>/jobs/<job id>/{clone,outcome}.
	DataDir string
	// DiskFree reads the free bytes of the data directory's volume; nil skips the
	// low-disk checks.
	DiskFree func() (uint64, error)
	// Secrets are values that must never reach a commit, a commit message or the
	// outcome; the device token is always one.
	Secrets []string
	// GitHub opens the pull request of a job that passed the host's checks. Nil
	// leaves the work on the Mac, as before; ServerURL is the board's address in
	// the pull request's footer.
	GitHub    GitHubAPI
	ServerURL string
	// PublishRetry is the pause between tries of the server's token and GitHub's
	// pull request call; zero means PublishRetry (two seconds).
	PublishRetry time.Duration
	// CheckInEvery and Minute are the check-in period and the unit of a repo's
	// time limit; zero means CheckInEvery and time.Minute. Tests shorten them.
	CheckInEvery time.Duration
	Minute       time.Duration
	// EventsEvery is how often the job's log is forwarded; zero means EventsEvery.
	EventsEvery time.Duration
	// PollEvery is how often a job's open pull request is read, and MaxWatch how
	// long it may stay unmerged before the job gives up; zero means PollEvery
	// (a minute) and MaxWatch (six hours).
	PollEvery time.Duration
	// RunningPollEvery is the read interval while checks run; zero means 20s.
	RunningPollEvery time.Duration
	MaxWatch         time.Duration
	// NoCIGrace is how long a pull request head may have no check runs before the
	// repo counts as having no CI and the job stops watching; zero means 5 minutes.
	NoCIGrace time.Duration
	// Preflight runs before each claim and reports whether the environment can run
	// a job (the Nix store readable); nil skips it. A failure opens the breaker
	// that holds claims until it passes again.
	Preflight func(context.Context) error
	// Sleep and Now are the clock a job's waits use; nil means sleepCtx and
	// time.Now. Tests replace them.
	Sleep func(context.Context, time.Duration)
	Now   func() time.Time
	Log   *slog.Logger

	// limitUntil is the unix time in nanoseconds a usage limit holds back new
	// claims until, shared by every running job.
	limitUntil atomic.Int64
	// fault is why the VM would not start, and faultUntil (unix nanoseconds) when
	// claims resume; a fault is the runner's, not the job's.
	fault      atomic.Pointer[string]
	faultUntil atomic.Int64
	// env is why the breaker holding claims for an environment fault is open, and
	// envProbed (unix nanoseconds) when the preflight last ran while it was.
	env       atomic.Pointer[string]
	envProbed atomic.Int64
	// running holds the ids of the jobs in flight, whose directories the sweep
	// leaves alone.
	running sync.Map
	// slots counts the claimed jobs whose goroutines have not ended: the loop's
	// own idea of how full it is.
	slots atomic.Int32
}

// RunningJobs are the ids of the jobs in flight.
func (r *JobRunner) RunningJobs() []string {
	var ids []string
	r.running.Range(func(k, _ any) bool {
		if id, ok := k.(string); ok {
			ids = append(ids, id)
		}
		return true
	})
	return ids
}

// LimitUntil is when the usage limit holding claims lifts; zero when none does.
func (r *JobRunner) LimitUntil() time.Time {
	n := r.limitUntil.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// MaxJobsNow is how many jobs may run at once.
func (r *JobRunner) MaxJobsNow() int { return r.maxJobs() }

// Reconcile frees job slots that outlived their job: the slot count and the
// Mac's busy count must not exceed the jobs actually in flight, or the loop and
// a pending update wait on jobs that are gone. It reports how many it freed.
func (r *JobRunner) Reconcile() int {
	live := int32(len(r.RunningJobs())) //nolint:gosec // a handful
	freed := 0
	for {
		cur := r.slots.Load()
		if cur <= live {
			break
		}
		if r.slots.CompareAndSwap(cur, live) {
			freed += int(cur - live)
			break
		}
	}
	if r.Activity != nil {
		freed += r.Activity.Reconcile(int(live))
	}
	return freed
}

// failed is a result for a job that never reached the agent.
func failed(outcome string) result {
	return result{
		succeeded: false, outcome: outcome, session: "", prURL: "",
		limited: false, resetsAt: time.Time{}, blocked: "", waitingOn: nil,
		finishedHarness: "", finishedModel: "",
		failure: "", retry: false, head: "", cause: "", times: nil, total: 0,
	}
}

// prepareOutcomeDir makes the directory the sandbox's non-root user writes the
// outcome into, whatever the umask.
func prepareOutcomeDir(dir string) error {
	if err := os.MkdirAll(dir, jobDirPerm); err != nil {
		return err //nolint:wrapcheck // the caller names it
	}
	return os.Chmod(
		dir,
		outcomeDirPerm,
	) //nolint:wrapcheck // the sandbox user writes here
}

// result is how a job ended, as the server is told.
type result struct {
	succeeded bool
	outcome   string
	session   string
	// prURL is the pull request opened for the job's branch, if one was.
	prURL string
	// limited says the harness stopped on its provider's usage limit, until resetsAt
	// when that is known.
	limited  bool
	resetsAt time.Time
	// blocked is what stopped the agent in the sandbox, for the board.
	blocked string
	// waitingOn is the tasks this one depends on, from the agent's Waiting-on line.
	waitingOn []string
	// finishedHarness and finishedModel name what finished the job after it
	// continued on the fallback; empty when the first harness did.
	finishedHarness string
	finishedModel   string
	// failure is why a finished attempt did not publish, for the agent's next
	// attempt; retry says another attempt may fix it. head is the branch tip the
	// host reviewed, "" when it never got that far.
	failure string
	retry   bool
	head    string
	// cause says which stage failed a retryable attempt: gate, push or ci.
	cause string
	// times and total are the job's phase times, set when its loop ends.
	times *phaseTimes
	total time.Duration
}

// maxJobs is how many jobs may run at once.
func (r *JobRunner) maxJobs() int {
	if r.MaxJobs == nil {
		return 1
	}
	return max(r.MaxJobs(), 1)
}

// holdClaims stops new claims until t: a usage limit is the account's, so every
// job would meet it. A later time wins.
func (r *JobRunner) holdClaims(t time.Time) {
	n := t.UnixNano()
	for {
		cur := r.limitUntil.Load()
		if n <= cur || r.limitUntil.CompareAndSwap(cur, n) {
			return
		}
	}
}

// claimsHeld says whether a usage limit or a runner fault still holds back new claims.
func (r *JobRunner) claimsHeld(now time.Time) bool {
	return now.UnixNano() < max(r.limitUntil.Load(), r.faultUntil.Load())
}

// holdFault stops new claims until t because the VM could not start, which
// every job would meet.
func (r *JobRunner) holdFault(reason string, t time.Time) {
	r.fault.Store(&reason)
	r.faultUntil.Store(t.UnixNano())
}

// clearFault notes that a VM started again.
func (r *JobRunner) clearFault() {
	r.fault.Store(nil)
	r.faultUntil.Store(0)
}

// Fault is why claims are held for a VM that would not start; empty when none is.
func (r *JobRunner) Fault() string {
	if f := r.envFault(); f != "" {
		return f
	}
	if p := r.fault.Load(); p != nil && r.now().UnixNano() < r.faultUntil.Load() {
		return *p
	}
	return ""
}

// Next claims one job and runs it to the end, reporting whether there was one.
func (r *JobRunner) Next(ctx context.Context) (bool, error) {
	job, err := r.claimJob(ctx, r.maxJobs())
	if err != nil || job == nil {
		return false, err
	}
	r.run(ctx, job)
	r.CleanSessions(ctx)
	r.CleanJobs(ctx)
	return true, nil
}

// Loop claims and runs up to MaxJobs jobs at once until ctx ends or the token is
// revoked, then waits for the running ones. The server holds an empty claim open,
// so a healthy loop makes one call per hold.
func (r *JobRunner) Loop(ctx context.Context) error {
	return r.loop(ctx, clock{sleep: sleepCtx, now: time.Now})
}

func (r *JobRunner) loop(ctx context.Context, clk clock) error {
	r.CleanSessions(ctx)
	r.CleanJobs(ctx)
	swept := clk.now()
	var (
		wg      sync.WaitGroup
		running = &r.slots
		// freed wakes the loop when a job ends and a slot opens.
		freed   = make(chan struct{}, 1)
		backoff = minLoop
	)
	defer wg.Wait()
	for ctx.Err() == nil {
		if clk.now().Sub(swept) >= sweepEvery {
			swept = clk.now()
			r.CleanSessions(ctx)
			r.CleanJobs(ctx)
		}
		limit := r.maxJobs()
		if (r.Paused != nil && r.Paused()) || r.claimsHeld(clk.now()) {
			clk.sleep(ctx, minLoop)
			continue
		}
		if int(running.Load()) >= limit {
			select {
			case <-freed:
			case <-time.After(minLoop):
			case <-ctx.Done():
			}
			continue
		}
		started := clk.now()
		job, err := r.claimJob(ctx, limit)
		switch {
		case err == nil:
			backoff = minLoop
			if job != nil {
				running.Add(1)
				wg.Go(func() { r.runClaimed(ctx, job, running, freed) })
			} else if wait := minLoop - clk.now().Sub(started); wait > 0 {
				clk.sleep(ctx, wait)
			}
		case ctx.Err() != nil:
			return nil
		case Unauthenticated(err):
			return ErrRevoked
		default:
			r.Log.Warn("claiming a job failed", "error", err, "retry_in", backoff)
			clk.sleep(ctx, backoff)
			backoff = min(backoff*backoffFactor, maxBackoff)
		}
	}
	return nil
}

// runClaimed runs a claimed job beside the others, then frees its slot.
func (r *JobRunner) runClaimed(
	ctx context.Context,
	job *ClaimedJob,
	running *atomic.Int32,
	freed chan<- struct{},
) {
	defer func() {
		running.Add(-1)
		select {
		case freed <- struct{}{}:
		default:
		}
	}()
	r.run(ctx, job)
	r.CleanSessions(ctx)
	r.CleanJobs(ctx)
}

func (r *JobRunner) run(ctx context.Context, job *ClaimedJob) {
	r.Log.Info("job claimed", "job", job.ID, "repo", job.RepoOwner+"/"+job.RepoName)
	if r.Activity != nil {
		if err := r.Activity.Begin(); err != nil {
			r.Log.Warn("could not hold the Mac awake", "error", err)
		}
		defer r.Activity.End()
	}

	r.running.Store(job.ID, struct{}{})
	if job.ChatID != "" {
		// The server finishes a chat's job when the chat ends, so no CompleteJob.
		r.runChat(ctx, job)
		r.running.Delete(job.ID)
		return
	}
	res := r.execute(ctx, job)
	r.running.Delete(job.ID)
	if res.limited && job.Routine == "" {
		r.keepClone(job)
	}
	if job.Routine != "" {
		// A routine has no task to resume.
		r.removeJobDir(job.ID)
	}

	// Told even when the runner is shutting down: the server would otherwise wait
	// five minutes to call the job lost.
	done, cancel := context.WithTimeout(context.WithoutCancel(ctx), completeTimeout)
	defer cancel()
	err := r.API.CompleteJob(done, r.Token, job.ID, JobEnd{
		Succeeded: res.succeeded, Outcome: res.outcome, SessionID: res.session,
		PRURL: res.prURL, Limited: res.limited, ResetsAt: res.resetsAt,
		Blocked: res.blocked, WaitingOn: res.waitingOn,
		FinishedHarness: res.finishedHarness, FinishedModel: res.finishedModel,
		Times: res.times, Total: res.total,
	})
	if err != nil {
		r.Log.Warn("could not report the job", "job", job.ID, "error", err)
	}
}

// check refuses a job the runner's own rules don't allow, whatever the server
// says: its id must be a UUID and its task id T-<number> or a UUID (both name
// directories), the harness one it can run, and the repo, branch and base inside
// the local allowlist.
func (r *JobRunner) check(job *ClaimedJob) error {
	if !jobIDPattern.MatchString(job.ID) {
		return fmt.Errorf("%w: invalid job id", ErrRefused)
	}
	if job.Routine != "" {
		if err := checkRoutine(job); err != nil {
			return err
		}
	} else if !taskIDPattern.MatchString(job.TaskID) {
		return fmt.Errorf("%w: invalid task id", ErrRefused)
	}
	if job.ResumeSessionID != "" && !sessionIDPattern.MatchString(job.ResumeSessionID) {
		return fmt.Errorf("%w: invalid session id", ErrRefused)
	}
	if err := r.checkHarness(job); err != nil {
		return err
	}
	allowed, err := r.Allowed()
	if err != nil {
		return fmt.Errorf("read the repo allowlist: %w", err)
	}
	return Validate(Job{
		Repo:       job.RepoOwner + "/" + job.RepoName,
		Branch:     job.Branch,
		BaseBranch: job.DefaultBranch,
	}, allowed)
}

// ModeClaude is the config mode that runs every job on Claude Code.
const ModeClaude = "claude"

// ModeOff is the config mode that claims nothing.
const ModeOff = "off"

// openCodeSessionPrefix starts every OpenCode session id; Claude Code's are UUIDs.
const openCodeSessionPrefix = "ses_"

// checkHarness applies this Mac's choice, whatever harness and model the job
// carries: nothing runs when it is off, and a Reply resumes only a session of the
// harness this Mac runs.
func (r *JobRunner) checkHarness(job *ClaimedJob) error {
	if r.Mode != nil && r.Mode() != ModeClaude {
		return fmt.Errorf("%w: this Mac's mode is off", ErrRefused)
	}
	mine := HarnessClaudeCode
	if oc, ok := r.Harness.(OpenCode); ok {
		mine = HarnessOpenCode
		if !validModel(oc.Model) {
			return fmt.Errorf("%w: invalid model", ErrRefused)
		}
	}
	if job.ResumeSessionID == "" {
		return nil
	}
	session := HarnessClaudeCode
	if strings.HasPrefix(job.ResumeSessionID, openCodeSessionPrefix) {
		session = HarnessOpenCode
	}
	if session != mine {
		return fmt.Errorf("%w: the session was started on %s, but this Mac runs %s",
			ErrRefused, session, mine)
	}
	return nil
}

// claimJob claims a job, reporting the harness and model this Mac runs. It
// claims nothing while that harness's credential is not stored: a job started
// without it would fail after using up the task.
func (r *JobRunner) claimJob(ctx context.Context, limit int) (*ClaimedJob, error) {
	if err := r.checkCredential(); err != nil {
		return nil, err
	}
	// A dead mount would use up the task: claim nothing until the preflight passes.
	if !r.envReady(ctx) {
		return nil, nil //nolint:nilnil // holding is not an error
	}
	if oc, ok := r.Harness.(OpenCode); ok {
		return r.API.ClaimJob(ctx, r.Token, limit, HarnessOpenCode, oc.Model)
	}
	return r.API.ClaimJob(ctx, r.Token, limit, HarnessClaudeCode, "")
}

// checkCredential reports an error when the harness's stored credential is missing.
func (r *JobRunner) checkCredential() error {
	if _, ok := r.Harness.(OpenCode); ok {
		if r.OpenRouterKey == nil {
			return nil
		}
		if _, err := r.OpenRouterKey(); err != nil {
			return fmt.Errorf("the OpenRouter key: %w", err)
		}
		return nil
	}
	if r.ClaudeToken == nil {
		return nil
	}
	if _, err := r.ClaudeToken(); err != nil {
		return fmt.Errorf("the Claude token: %w", err)
	}
	return nil
}

// harnessFor is the harness a job runs on: this Mac's, not the job's.
func (r *JobRunner) harnessFor(*ClaimedJob) Harness {
	return r.Harness
}

func (r *JobRunner) minute() time.Duration {
	return cmp.Or(r.Minute, time.Minute)
}

// limitFor is how long a job may run. The server is untrusted: its number can
// neither stretch the limit past a day nor overflow it.
func (r *JobRunner) limitFor(job *ClaimedJob) time.Duration {
	if job.TimeLimitMinutes <= 0 {
		return DefaultTimeLimit
	}
	return time.Duration(min(job.TimeLimitMinutes, maxTimeLimitMinutes)) * r.minute()
}

// execute runs one job to its end: its attempts and then its pull request, in one
// VM (jobLoop.run). It touches nothing shared but the runner's configuration, so
// several can run side by side.
func (r *JobRunner) execute(ctx context.Context, job *ClaimedJob) result {
	if err := r.check(job); err != nil {
		return failed("Refused by the runner: " + err.Error())
	}

	dirs, since, err := r.prepareDirs(ctx, job)
	if err != nil {
		return failed("Could not " + err.Error())
	}
	launch, err := r.launchOf(job)
	if err != nil {
		return failed("Could not " + err.Error())
	}

	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tap := &sessionTap{
		harness: r.harnessFor(job),
		mu:      sync.Mutex{},
		partial: nil,
		session: "",
		limited: false, resetsAt: time.Time{},
		transient: false, tail: nil,
	}
	secrets := append(r.secrets(), launch.secrets...)
	events := newEventTap(secrets)
	var stopped atomic.Bool
	wait := r.watch(jobCtx, cancel, job.ID, tap, events, &stopped)

	loop := &jobLoop{
		r: r, job: job, dirs: dirs, since: since, launch: launch, secrets: secrets,
		ctx: jobCtx, parent: ctx, tap: tap, events: events, stopped: &stopped,
		session: nil, resume: job.ResumeSessionID, limit: r.limitFor(job),
		token: "", tokenAt: time.Time{},
		times:   phaseTimes{agent: 0, gates: 0, publish: 0, ci: 0, merge: 0, retries: nil},
		greenAt: time.Time{}, began: time.Time{}, causes: nil,
		sandboxFixed: false, sandboxOpened: false, startRef: "",
	}
	var res result
	if prompt, ok := routinePrompt(job.Routine); ok {
		res = loop.runRoutine(prompt)
	} else {
		res = loop.run()
	}
	cancel()
	wait()
	if res.prURL == "" && !res.limited && res.cause != causePush {
		if line := r.pushStranded(ctx, job, dirs.clone, since, secrets); line != "" {
			res.outcome = strings.TrimSpace(res.outcome + "\n\n" + line)
		}
	}
	if launch.mcpNote != "" {
		res.outcome = strings.TrimSpace(launch.mcpNote + "\n\n" + res.outcome)
	}
	if job.Routine != "" && !job.hasBoard() {
		res.outcome = strings.TrimSpace(
			"The board tools (factory_*) were unavailable: the job had no board account.\n\n" +
				res.outcome)
	}
	// What the agent said last is what you most want to read.
	r.flush(ctx, job.ID, events)
	return res
}

// noOutcomeBlock is the report for a job that ended before the agent said
// anything: the reason, unless the agent already named what blocked it.
func noOutcomeBlock(text, reason string, secrets []string, have string) string {
	if text != "" || have != "" || leaks(reason, secrets) {
		return have
	}
	return reason
}

// jobDirs are a job's directories on the host.
type jobDirs struct {
	clone, outcome, session string
	// note says where the clone's work came from when that was not the usual way.
	note string
	// taken is set when the clone is earlier work taken over from this Mac.
	taken bool
}

// prepareDirs clears the job's directory, makes its clone and the outcome and
// session directories, and returns where a Reply's host checks start. An error says
// which step failed.
func (r *JobRunner) prepareDirs(
	ctx context.Context,
	job *ClaimedJob,
) (jobDirs, string, error) {
	dir := filepath.Join(r.DataDir, "jobs", job.ID)
	dirs := jobDirs{
		clone:   filepath.Join(dir, "clone"),
		outcome: filepath.Join(dir, "outcome"),
		// The session state belongs to the task, not the job: a Reply resumes it.
		session: filepath.Join(r.DataDir, "sessions", job.TaskID),
		note:    "", taken: false,
	}
	if job.Routine != "" {
		// A routine has no task, and nothing to resume.
		dirs.session = filepath.Join(dir, "session")
	}
	// A job claimed again (after a restart) starts from a clean directory, except
	// for a clone holding work that was never pushed.
	kept := r.setAsideClone(job, dirs.clone)
	if err := removeAll(dir); err != nil {
		return jobDirs{}, "", fmt.Errorf("clear the job directory: %w", err)
	}
	since, err := r.prepareClone(ctx, job, &dirs, kept)
	if kept != "" {
		_ = removeAll(kept)
	}
	if err != nil {
		return jobDirs{}, "", fmt.Errorf("prepare the clone: %w", err)
	}
	if job.Routine == "" {
		// Lets the sweep ask whether the job's task is done.
		// Without it the directory goes after staleJobAge instead.
		_ = os.WriteFile(filepath.Join(dir, taskMarker), []byte(job.TaskID), 0o600)
	}
	named := map[string]string{"outcome": dirs.outcome, "session": dirs.session}
	for name, d := range named {
		if err = prepareOutcomeDir(d); err != nil {
			return jobDirs{}, "", fmt.Errorf("prepare the %s directory: %w", name, err)
		}
	}
	return dirs, since, nil
}

// launch is what a job starts with: its command, environment and egress proxy.
type launch struct {
	command []string
	env     map[string]string
	proxy   ProxyListener
	// secrets are this job's own credentials, which must never reach a commit, a
	// log line or the outcome.
	secrets []string
	// mcpNote names the device MCP servers the job should have had but could not.
	mcpNote string
	// mcp are the device MCP servers the job got.
	mcp []MCPServer
}

// launchOf gathers a job's launch inputs; an error says why it cannot start.
func (r *JobRunner) launchOf(job *ClaimedJob) (launch, error) {
	command, env, err := r.invocation(job)
	if err != nil {
		return launch{}, fmt.Errorf("prepare the job: %w", err)
	}
	secrets := []string{job.MCPClientSecret}
	if token := env["CLAUDE_CODE_OAUTH_TOKEN"]; token != "" {
		// The token is in the VM only to be sent to Anthropic through the proxy;
		// without one it would sit there for nothing.
		if r.Proxy == nil {
			return launch{}, errors.New("prepare the job: a Claude token needs the egress proxy")
		}
		secrets = append(secrets, token)
	}
	var proxy ProxyListener
	var note string
	var mcp []MCPServer
	if r.Proxy != nil {
		mcp, note = r.deviceMCP(job, env)
		if proxy, err = r.Proxy(job, mcp); err != nil {
			return launch{}, fmt.Errorf("prepare the egress proxy: %w", err)
		}
	}
	return launch{
		command: command, env: env, proxy: proxy, secrets: secrets, mcpNote: note,
		mcp: mcp,
	}, nil
}

// deviceMCP is the device's MCP servers the job gets, named in its environment. A
// server that is not signed in, or a config that cannot be read, does not stop the
// job: it is logged once and returned as a note for the outcome.
func (r *JobRunner) deviceMCP(
	job *ClaimedJob, env map[string]string,
) ([]MCPServer, string) {
	if r.MCP == nil {
		return nil, ""
	}
	servers, missing, err := r.MCP()
	var note string
	switch {
	case err != nil:
		servers = nil
		note = "The device's MCP servers could not be read: " + err.Error()
		r.Log.Warn("device MCP servers unavailable", "job", job.ID, "error", err)
	case len(missing) > 0:
		slices.Sort(missing)
		note = "MCP servers not signed in, so the job ran without them: " +
			strings.Join(missing, ", ") + " (wardcase mcp login <name>)"
		r.Log.Warn("device MCP servers not signed in", "job", job.ID, "servers", missing)
	}
	names := make([]string, 0, len(servers))
	for _, s := range servers {
		names = append(names, s.Name)
	}
	slices.Sort(names)
	if len(names) > 0 {
		env[MCPServersEnv] = strings.Join(names, " ")
	}
	return servers, note
}

// invocation is what the sandbox runs for the job, and its environment: a fresh
// session, or a Reply's resumed one.
func (r *JobRunner) invocation(
	job *ClaimedJob,
) ([]string, map[string]string, error) {
	harness := r.harnessFor(job)
	command := harness.Command()
	if job.ResumeSessionID != "" {
		command = harness.Resume(job.ResumeSessionID)
	}
	env := harness.SessionEnv(SandboxSessionDir)
	env["FACTORY_JOB"] = job.ID
	env["WARDCASE_RUN"] = job.ID
	env["FACTORY_OUTCOME_PATH"] = SandboxOutcomePath
	author := cmp.Or(job.MCPClientID, "factory")
	email := cmp.Or(r.AuthorEmail, DefaultBotEmail)
	env["GIT_AUTHOR_NAME"], env["GIT_COMMITTER_NAME"] = author, author
	env["GIT_AUTHOR_EMAIL"], env["GIT_COMMITTER_EMAIL"] = email, email
	if _, claude := harness.(ClaudeCode); claude && r.ClaudeToken != nil {
		token, err := r.ClaudeToken()
		if err != nil {
			return nil, nil, fmt.Errorf("the Claude token: %w", err)
		}
		env["CLAUDE_CODE_OAUTH_TOKEN"] = token
	}
	return command, env, nil
}

// prepareClone makes the job's clone: the task's existing branch when it was
// pushed, else the work an earlier job left in a clone on this Mac, else a new
// branch off the base. It also returns the commit an existing branch began at ("" for
// a new one or a taken-over clone), where the host's checks start.
func (r *JobRunner) prepareClone(
	ctx context.Context,
	job *ClaimedJob,
	dirs *jobDirs,
	kept string,
) (string, error) {
	cloneDir := dirs.clone
	if r.takeClone(job, cloneDir) {
		r.commitTaken(ctx, job, cloneDir)
		return "", nil
	}
	tip, found, err := r.Git.RestoreIfPushed(
		ctx, job.RepoOwner, job.RepoName, job.Branch, cloneDir)
	if err != nil {
		return "", fmt.Errorf("restore the branch %s: %w", job.Branch, err)
	}
	if found {
		return tip, nil
	}
	if r.takeStranded(job, cloneDir, kept) == nil {
		dirs.note, dirs.taken = noteFromMac, true
		r.commitTaken(ctx, job, cloneDir)
		return "", nil
	}
	if job.ResumeSessionID != "" {
		dirs.note = noteFresh
	}
	return "", r.Git.Clone(ctx, job.RepoOwner, job.RepoName, job.DefaultBranch,
		job.Branch, cloneDir)
}

// commitTaken commits a taken-over clone's uncommitted tracked edits; a failure
// is logged and the clone used as it is.
func (r *JobRunner) commitTaken(ctx context.Context, job *ClaimedJob, cloneDir string) {
	if err := r.Git.CommitTracked(ctx, cloneDir); err != nil {
		r.Log.Warn("could not commit a taken-over clone's edits", "job", job.ID,
			"error", err.Error())
	}
}

// limitMarker names the file recording which job's clone a limited task left behind.
func (r *JobRunner) limitMarker(taskID string) string {
	return filepath.Join(r.DataDir, "limited", taskID)
}

// keepClone records the clone of a job a usage limit stopped, so the job that
// continues the task takes its unpushed work over.
func (r *JobRunner) keepClone(job *ClaimedJob) {
	marker := r.limitMarker(job.TaskID)
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(marker, []byte(job.ID), 0o600)
}

// takeClone moves the clone a limited job of the same task left to cloneDir, so the
// work it never pushed carries on; false when there is none to take.
func (r *JobRunner) takeClone(job *ClaimedJob, cloneDir string) bool {
	marker := r.limitMarker(job.TaskID)
	data, err := os.ReadFile(marker)
	if err != nil {
		return false
	}
	_ = os.Remove(marker)
	prevID := strings.TrimSpace(string(data))
	if !jobIDPattern.MatchString(prevID) || prevID == job.ID {
		return false
	}
	prev := filepath.Join(r.DataDir, "jobs", prevID, "clone")
	//nolint:gosec // prevID is a UUID
	if _, err = os.Stat(filepath.Join(prev, ".git")); err != nil {
		return false
	}
	if err = os.MkdirAll(filepath.Dir(cloneDir), 0o700); err != nil {
		return false
	}
	return os.Rename(prev, cloneDir) == nil //nolint:gosec // prevID is a UUID
}

// publishable runs the host's checks on a job the agent says it finished. A job
// that fails them is failed, with the reason ahead of the agent's own outcome. One
// that passes is pushed and gets its pull request, when pushing is set up.
func (r *JobRunner) publishable(
	ctx context.Context,
	job *ClaimedJob,
	cloneDir, since, text string,
	res result,
	secrets []string,
) result {
	reviewed, err := r.Git.Inspect(ctx, job.RepoOwner, job.RepoName, job.DefaultBranch,
		job.Branch, job.ID, cloneDir, since, secrets)
	if err == nil && leaks(text, secrets) {
		reviewed.Close()
		err = fmt.Errorf("%w: the outcome contains a credential value", ErrHostCheck)
	}
	if err != nil {
		why := strings.TrimPrefix(err.Error(), ErrHostCheck.Error()+": ")
		res.outcome = "Not published: " + why
		if !leaks(text, secrets) {
			res.outcome += "\n\n" + text
		}
		res.failure, res.retry, res.cause = why, true, "gate"
		return res
	}
	defer reviewed.Close()

	res.succeeded = true
	res.outcome = text
	if r.GitHub == nil {
		return res
	}
	res.head = reviewed.Head(ctx)
	if err = checksFailure(text); err != nil {
		why := strings.TrimPrefix(err.Error(), errChecks.Error()+": ")
		res.outcome = "Not published: " + why + "\n\n" + text
		res.failure, res.retry, res.cause = why, true, "gate"
		return res
	}
	prURL, why, final := r.publish(ctx, job, reviewed, text)
	if why != "" {
		// The agent's work is fine, so the job still succeeded; the task just goes
		// to Needs you with the branch on the Mac, and the reason ahead of the text.
		res.outcome = "Not published: " + why + "\n\n" + text
		res.failure, res.retry, res.cause = why, !final, causePush
		return res
	}
	res.prURL = prURL
	return res
}

// publish pushes the reviewed branch and opens its pull request. It returns the
// pull request's address, or why that didn't happen: no secret is ever in it.
// final says another attempt cannot change that.
func (r *JobRunner) publish(
	ctx context.Context,
	job *ClaimedJob,
	reviewed *Reviewed,
	text string,
) (string, string, bool) {
	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()

	token, err := r.pushToken(ctx, job.ID)
	if PushUnavailable(err) {
		return "", "pushing is not set up for this repo, so the work stays in the " +
			"Mac's clone.", true
	}
	if err != nil || token == "" {
		r.Log.Warn("could not get a push token", "job", job.ID, "error", err)
		return "", "could not get a push token from the server, so the work stays " +
			"in the Mac's clone.", false
	}
	remote := r.Git.RemoteURL(job.RepoOwner, job.RepoName)
	if err = reviewed.Push(ctx, remote, token); err != nil {
		r.Log.Warn("push failed", "job", job.ID, "error", scrub(err.Error(), token))
		return "", "the branch could not be pushed (it may already hold other work " +
				"on GitHub, which is never overwritten), so the work stays in the Mac's clone.",
			false
	}
	title, body := pullRequestText(text, job, r.ServerURL)
	prURL, err := r.openPullRequest(ctx, token, PullRequest{
		Owner: job.RepoOwner, Repo: job.RepoName, Head: job.Branch,
		Base: job.DefaultBranch, Title: title, Body: body,
	})
	if err != nil {
		r.Log.Warn("pull request failed", "job", job.ID, "error", scrub(err.Error(), token))
		return "", "the branch was pushed, but its pull request could not be opened.", false
	}
	return prURL, "", false
}

// secrets are the values that must not leave the Mac in a log, commit or outcome.
func (r *JobRunner) secrets() []string {
	return append([]string{r.Token}, r.Secrets...)
}

// leaks reports whether text contains any secret long enough to search for.
func leaks(text string, secrets []string) bool {
	for _, secret := range secrets {
		if len(secret) >= minSecretLen && strings.Contains(text, secret) {
			return true
		}
	}
	return false
}

// watch starts the check-ins and the log forwarding of a running job, and
// returns what waits for both to end once ctx does.
func (r *JobRunner) watch(
	ctx context.Context,
	cancel context.CancelFunc,
	jobID string,
	tap *sessionTap,
	events *eventTap,
	stopped *atomic.Bool,
) func() {
	var wg sync.WaitGroup
	wg.Go(func() { r.checkIns(ctx, cancel, jobID, tap, stopped) })
	wg.Go(func() { r.forwardEvents(ctx, jobID, events) })
	return wg.Wait
}

// checkIns reports the job alive until ctx ends, and cancels the run when the
// server says it is no longer running. A failed check-in is only logged: the
// server decides when silence means lost.
func (r *JobRunner) checkIns(
	ctx context.Context,
	cancel context.CancelFunc,
	jobID string,
	tap *sessionTap,
	stopped *atomic.Bool,
) {
	ticker := time.NewTicker(cmp.Or(r.CheckInEvery, CheckInEvery))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stop, err := r.API.CheckIn(ctx, r.Token, jobID, tap.Session())
			if err != nil {
				if ctx.Err() == nil {
					r.Log.Warn("check-in failed", "job", jobID, "error", err)
				}
				continue
			}
			if stop {
				stopped.Store(true)
				cancel()
				return
			}
		}
	}
}

// readOutcome reads what the agent wrote, capped; a missing file is "". The agent
// controls that directory, so only a regular file counts: a symlink to a host
// file, a FIFO or a device is ignored, and a huge file is read only up to the cap.
func readOutcome(path string) string {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxOutcomeBytes))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ToValidUTF8(string(data), ""))
}

// limitReason is the outcome's first line for a job a usage limit stopped.
func limitReason(resetsAt time.Time) string {
	if resetsAt.IsZero() {
		return "Stopped: the Claude usage limit was reached."
	}
	return "Stopped: the Claude usage limit was reached; it resets at " +
		resetsAt.UTC().Format("2006-01-02 15:04 MST") + "."
}

// sessionTap watches the harness's output for its session id, for a usage limit and
// for a provider it could not reach, and keeps the last lines of it for a report.
type sessionTap struct {
	harness  Harness
	mu       sync.Mutex
	partial  []byte
	session  string
	limited  bool
	resetsAt time.Time
	// transient is set when the harness reported a provider transport failure.
	transient bool
	// tail is the last lines of output, for the block a failed attempt files.
	tail []string
}

const (
	// tailLines is how many of the harness's last lines a failure keeps.
	tailLines = 8
	// tailLine is how much of one long line that report keeps, from its end.
	tailLine = 300
)

// Write implements io.Writer: it splits the output into lines and keeps the
// first session id any line carries.
func (t *sessionTap) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.partial = append(t.partial, p...)
	for {
		i := bytes.IndexByte(t.partial, '\n')
		if i < 0 {
			break
		}
		line := t.partial[:i]
		if t.session == "" {
			t.session = t.harness.SessionID(line)
		}
		if hit, at := t.harness.LimitFrom(line); hit {
			t.limited = true
			if !at.IsZero() {
				t.resetsAt = at
			}
		}
		if t.harness.TransientError(line) {
			t.transient = true
		}
		t.keep(line)
		t.partial = t.partial[i+1:]
	}
	if len(t.partial) > maxTapLine {
		t.partial = nil
	}
	return len(p), nil
}

// keep remembers one line of output, capped and beyond that its end, where the
// harness puts the error.
func (t *sessionTap) keep(line []byte) {
	if len(line) == 0 {
		return
	}
	s := string(line)
	if len(s) > tailLine {
		s = "…" + s[len(s)-tailLine:]
	}
	t.tail = append(t.tail, s)
	if len(t.tail) > tailLines {
		t.tail = t.tail[len(t.tail)-tailLines:]
	}
}

// Limit reports whether a usage limit was seen, and when it lifts (zero if unsaid).
func (t *sessionTap) Limit() (bool, time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	limited, resetsAt := t.limited, t.resetsAt
	// The harness's last line may end without a newline.
	if hit, at := t.harness.LimitFrom(t.partial); hit {
		limited = true
		if !at.IsZero() {
			resetsAt = at
		}
	}
	return limited, resetsAt
}

// Transient reports whether the harness said it could not reach its provider.
func (t *sessionTap) Transient() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.transient || t.harness.TransientError(t.partial)
}

// Tail is the last lines of the harness's output.
func (t *sessionTap) Tail() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.tail)
}

// resetLimit forgets a usage limit seen, for the next attempt.
func (t *sessionTap) resetLimit() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.limited, t.resetsAt, t.transient, t.tail, t.partial = false,
		time.Time{}, false, nil, nil
}

// resetTransient forgets a provider failure seen, for the next run of one attempt.
func (t *sessionTap) resetTransient() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.transient, t.tail = false, nil
}

// Session is the session id seen so far, or "".
func (t *sessionTap) Session() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.session
}
