package runner

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
)

var jobIDPattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

// preamble is the runner's own, repo-agnostic part of every prompt; the server
// can't change it. %s is the job's branch.
const preamble = `You are running unattended in a sandbox with no network ` +
	`except the model. ` +
	`There is no GitHub: do not try to push or open a pull request. Commit your ` +
	`work locally on the current branch (%s). When you are done, write a short ` +
	`outcome (what you did, what is left, the PR title and body you would use) ` +
	`to the file named by $FACTORY_OUTCOME_PATH. Nobody can answer questions ` +
	`while you run.`

// JobRunner claims jobs from the server and runs each one in a sandbox.
type JobRunner struct {
	API     JobAPI
	Token   string
	Allowed func() ([]string, error)
	Git     *Git
	Sandbox Sandbox
	Harness Harness
	// Activity, when set, holds the Mac awake and blocks self-update while a job runs.
	Activity *Activity
	// DataDir holds <DataDir>/jobs/<job id>/{clone,outcome}.
	DataDir string
	// CheckInEvery and Minute are the check-in period and the unit of a repo's
	// time limit; zero means CheckInEvery and time.Minute. Tests shorten them.
	CheckInEvery time.Duration
	Minute       time.Duration
	Log          *slog.Logger
}

// failed is a result for a job that never reached the agent.
func failed(outcome string) result {
	return result{succeeded: false, outcome: outcome, session: ""}
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
}

// Next claims one job and runs it to the end, reporting whether there was one.
func (r *JobRunner) Next(ctx context.Context) (bool, error) {
	job, err := r.API.ClaimJob(ctx, r.Token)
	if err != nil || job == nil {
		return false, err
	}
	r.run(ctx, job)
	return true, nil
}

// Loop claims and runs jobs until ctx ends or the token is revoked. The server
// holds an empty claim open, so a healthy loop makes one call per hold.
func (r *JobRunner) Loop(ctx context.Context) error {
	return r.loop(ctx, clock{sleep: sleepCtx, now: time.Now})
}

func (r *JobRunner) loop(ctx context.Context, clk clock) error {
	backoff := minLoop
	for ctx.Err() == nil {
		started := clk.now()
		ran, err := r.Next(ctx)
		switch {
		case err == nil:
			backoff = minLoop
			if wait := minLoop - clk.now().Sub(started); !ran && wait > 0 {
				clk.sleep(ctx, wait)
			}
		case ctx.Err() != nil:
			return nil //nolint:nilerr // a stop is not a failure
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

func (r *JobRunner) run(ctx context.Context, job *ClaimedJob) {
	r.Log.Info("job claimed", "job", job.ID, "repo", job.RepoOwner+"/"+job.RepoName)
	if r.Activity != nil {
		if err := r.Activity.Begin(); err != nil {
			r.Log.Warn("could not hold the Mac awake", "error", err)
		}
		defer r.Activity.End()
	}

	res := r.execute(ctx, job)

	// Told even when the runner is shutting down: the server would otherwise wait
	// five minutes to call the job lost.
	done, cancel := context.WithTimeout(context.WithoutCancel(ctx), completeTimeout)
	defer cancel()
	err := r.API.CompleteJob(
		done,
		r.Token,
		job.ID,
		res.succeeded,
		res.outcome,
		res.session,
	)
	if err != nil {
		r.Log.Warn("could not report the job", "job", job.ID, "error", err)
	}
}

// check refuses a job the runner's own rules don't allow, whatever the server
// says: its id must be a UUID (it names a directory), the harness one it can run,
// and the repo, branch and base inside the local allowlist.
func (r *JobRunner) check(job *ClaimedJob) error {
	if !jobIDPattern.MatchString(job.ID) {
		return fmt.Errorf("%w: invalid job id", ErrRefused)
	}
	if job.Harness != HarnessClaudeCode {
		return fmt.Errorf("%w: harness %q is not supported yet", ErrRefused, job.Harness)
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

func (r *JobRunner) execute(ctx context.Context, job *ClaimedJob) result {
	if err := r.check(job); err != nil {
		return failed("Refused by the runner: " + err.Error())
	}

	dir := filepath.Join(r.DataDir, "jobs", job.ID)
	cloneDir := filepath.Join(dir, "clone")
	outcomeDir := filepath.Join(dir, "outcome")
	err := r.Git.Clone(ctx, job.RepoOwner, job.RepoName, job.DefaultBranch,
		job.Branch, cloneDir)
	if err != nil {
		return failed("Could not prepare the clone: " + err.Error())
	}
	if err = prepareOutcomeDir(outcomeDir); err != nil {
		return failed("Could not prepare the outcome directory: " + err.Error())
	}

	minute := cmp.Or(r.Minute, time.Minute)
	limit := DefaultTimeLimit
	if job.TimeLimitMinutes > 0 {
		limit = time.Duration(job.TimeLimitMinutes) * minute
	}
	runCtx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	tap := &sessionTap{harness: r.Harness, mu: sync.Mutex{}, partial: nil, session: ""}
	var stopped atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.checkIns(runCtx, cancel, job.ID, tap, &stopped)
	}()

	exit, runErr := r.Sandbox.Run(runCtx, SandboxSpec{
		CloneDir:   cloneDir,
		OutcomeDir: outcomeDir,
		Env: map[string]string{
			"FACTORY_JOB":          job.ID,
			"FACTORY_OUTCOME_PATH": SandboxOutcomePath,
		},
		SetupCommands: job.SetupCommands,
		Command:       r.Harness.Command(),
		Stdin:         fmt.Sprintf(preamble, job.Branch) + "\n\n" + job.Prompt,
		Log:           tap,
	})
	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)
	cancel()
	wg.Wait()

	text := readOutcome(filepath.Join(outcomeDir, outcomeFile))
	res := result{succeeded: false, outcome: "", session: tap.Session()}
	var reason string
	switch {
	case stopped.Load():
		reason = "Stopped: the server no longer counts this job as running."
	case timedOut:
		reason = fmt.Sprintf("Timed out after %d minutes.", job.TimeLimitMinutes)
	case ctx.Err() != nil:
		reason = "The runner shut down while the job ran."
	case runErr != nil:
		reason = "The sandbox could not run the job: " + runErr.Error()
	case exit != 0:
		reason = fmt.Sprintf("The agent exited with status %d.", exit)
	default:
		res.succeeded = true
		res.outcome = text
		return res
	}
	res.outcome = strings.TrimSpace(reason + "\n\n" + text)
	return res
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
				r.Log.Warn("check-in failed", "job", jobID, "error", err)
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

// readOutcome reads what the agent wrote, capped; a missing file is "".
func readOutcome(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(data) > maxOutcomeBytes {
		data = data[:maxOutcomeBytes]
	}
	return strings.TrimSpace(strings.ToValidUTF8(string(data), ""))
}

// sessionTap watches the harness's output for its session id.
type sessionTap struct {
	harness Harness
	mu      sync.Mutex
	partial []byte
	session string
}

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
		if t.session == "" {
			t.session = t.harness.SessionID(t.partial[:i])
		}
		t.partial = t.partial[i+1:]
	}
	if len(t.partial) > maxTapLine {
		t.partial = nil
	}
	return len(p), nil
}

// Session is the session id seen so far, or "".
func (t *sessionTap) Session() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.session
}
