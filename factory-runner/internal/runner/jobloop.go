package runner

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const (
	// DefaultPollEvery is how often a job's open pull request is read.
	DefaultPollEvery = time.Minute
	// DefaultMaxWatch is how long a pull request may stay unmerged before the job
	// gives up on it.
	DefaultMaxWatch = 6 * time.Hour

	// maxAttempts bounds the agent runs of one job, whatever else happens.
	maxAttempts = 20
	// defaultLimitWait is how long a usage limit with no reset time is waited out.
	defaultLimitWait = time.Hour
	maxLimitWait     = 7 * 24 * time.Hour
	// tokenLife is how long a push token is reused for reading the pull request.
	tokenLife   = 20 * time.Minute
	noteTimeout = 15 * time.Second
	maxWhy      = 300

	limitLifted = "The usage limit has lifted. Carry on where you stopped, and " +
		"write the outcome when you are done."
)

// jobLoop is one job's whole run in one VM: agent attempts (the first on the
// job's prompt, then resumed with what failed), the host's checks and the push
// after each, and the pull request watched until it merges. It holds no state
// shared with another job, so several can run side by side.
type jobLoop struct {
	r    *JobRunner
	job  *ClaimedJob
	dirs jobDirs
	// since is where a Reply's host checks start.
	since   string
	launch  launch
	secrets []string
	// ctx ends when the board stops the job; parent also ends when the runner
	// shuts down.
	ctx     context.Context //nolint:containedctx // the loop's own lifetime
	parent  context.Context //nolint:containedctx // the loop's own lifetime
	tap     *sessionTap
	events  *eventTap
	stopped *atomic.Bool
	session SandboxSession
	// limit bounds one attempt.
	limit   time.Duration
	token   string
	tokenAt time.Time
}

// step is how an attempt ended: the job's result when that is the end, or what
// to tell the agent next.
type step struct {
	done bool
	res  result
	// feedback is the next attempt's prompt, and why the short reason for the
	// board; sig and head identify the failure, for spotting no progress.
	feedback string
	why      string
	sig      string
	head     string
	// waitUntil is set when a usage limit stopped the attempt.
	limited   bool
	waitUntil time.Time
}

// over is a step that ends the job with res.
func over(res result) step {
	return step{
		done: true, res: res, feedback: "", why: "", sig: "", head: "",
		limited: false, waitUntil: time.Time{},
	}
}

// again is a step that asks the agent for another attempt.
func again(res result, why, sig, head, feedback string) step {
	return step{
		done: false, res: res, feedback: feedback, why: why, sig: sig, head: head,
		limited: false, waitUntil: time.Time{},
	}
}

func (l *jobLoop) now() time.Time {
	if l.r.Now != nil {
		return l.r.Now()
	}
	return time.Now()
}

func (l *jobLoop) sleep(d time.Duration) {
	if l.r.Sleep != nil {
		l.r.Sleep(l.ctx, d)
		return
	}
	sleepCtx(l.ctx, d)
}

// run is the job from its first prompt to its end.
func (l *jobLoop) run() result {
	spec := SandboxSpec{
		CloneDir: l.dirs.clone, OutcomeDir: l.dirs.outcome, SessionDir: l.dirs.session,
		Env: l.launch.env, SetupCommands: l.job.SetupCommands, Command: nil,
		Proxy: l.launch.proxy, Stdin: "", Log: nil,
	}
	session, err := openSession(l.ctx, l.r.Sandbox, spec)
	if err != nil {
		reason := "The sandbox could not run the job: " + err.Error()
		res := failed(reason)
		res.session = l.tap.Session()
		res.blocked = noOutcomeBlock("", reason, l.secrets, "")
		return res
	}
	defer session.Close()
	l.session = session

	command := l.launch.command
	prompt := fmt.Sprintf(preamble, l.job.Branch) + "\n\n" + l.job.Prompt
	var last *step
	for attempt := 1; ; attempt++ {
		s := l.attempt(command, prompt)
		if s.done {
			return s.res
		}
		if attempt >= maxAttempts {
			return l.stop(s, fmt.Sprintf("Stopped after %d attempts.", attempt))
		}
		if !s.limited {
			if last != nil && last.sig == s.sig && last.head == s.head {
				return l.stop(s, "Stopped: no progress. The same failure came back with "+
					"no new commit ("+s.why+").")
			}
			last = &s
		}
		sid := l.tap.Session()
		if sid == "" {
			return l.stop(s, "Stopped: the agent has no session to resume.")
		}
		if !s.limited {
			l.note(fmt.Sprintf("Attempt %d: %s, retrying", attempt+1, s.why))
		} else {
			end := l.limitEnd(s.waitUntil)
			l.note(fmt.Sprintf("Attempt %d: %s, waiting until %s, then resuming",
				attempt+1, s.why, end.UTC().Format("2006-01-02 15:04 MST")))
			l.waitOutLimit(end)
			if why := l.ended(); why != "" {
				return l.stop(s, why)
			}
		}
		command, prompt = l.r.harnessFor(l.job).Resume(sid), s.feedback
	}
}

// stop ends the job on a failed attempt, with the reason ahead of the agent's text.
func (l *jobLoop) stop(s step, reason string) result {
	res := s.res
	// An open pull request that cannot go on is not a success, whatever the
	// agent's work was; without one, the host's verdict on the work stands.
	res.succeeded = res.succeeded && res.prURL == ""
	res.limited = s.limited
	if s.limited {
		res.resetsAt = s.waitUntil
	}
	res.outcome = strings.TrimSpace(reason + "\n\n" + res.outcome)
	return res
}

// ended says why the job must stop now, or "" while it may go on.
func (l *jobLoop) ended() string {
	switch {
	case l.stopped.Load():
		return "Stopped: the server no longer counts this job as running."
	case l.parent.Err() != nil:
		return "The runner shut down while the job ran."
	}
	return ""
}

// attempt runs the agent once, then checks and publishes what it left, and, once
// the pull request is open, watches it.
func (l *jobLoop) attempt(command []string, prompt string) step {
	text, end := l.agent(command, prompt)
	if end != nil {
		return *end
	}
	res := l.baseResult()
	if b, ok := findLine(text, "Blocked:"); ok && !strings.EqualFold(b, "none") &&
		!leaks(b, l.secrets) {
		res.blocked = b
	}
	res = l.r.publishable(l.parent, l.job, l.dirs.clone, l.since, text, res, l.secrets)
	if res.head == "" {
		// Only an identity for spotting no progress, so the agent's own ref file
		// will do where the host never got to review the branch.
		ref, _ := os.ReadFile(filepath.Join(l.dirs.clone, ".git", "refs", "heads",
			l.job.Branch))
		res.head = strings.TrimSpace(string(ref))
	}
	switch {
	case res.blocked != "":
		return over(res)
	case res.prURL != "":
		return l.watch(res, text)
	case res.retry:
		return again(res, "not published: "+short(res.failure),
			"publish:"+res.failure, res.head,
			"Your last attempt produced no pull request: "+res.failure+
				"\n\nFix that, run the gates again, commit, and write the outcome "+
				"again (the first line the pull request title, the last the Checks: "+
				"line). Your work continues until its pull request merges.")
	}
	return over(res)
}

func (l *jobLoop) baseResult() result {
	res := failed("")
	res.session = l.tap.Session()
	return res
}

// agent runs the agent once. It returns the outcome the agent wrote, or the step
// that ends or pauses the job when there is none worth publishing.
func (l *jobLoop) agent(command []string, prompt string) (string, *step) {
	attemptCtx, cancel := context.WithTimeout(l.ctx, l.limit)
	defer cancel()
	l.tap.resetLimit()
	// An earlier attempt's outcome must not pass for this one's.
	_ = os.Remove(filepath.Join(l.dirs.outcome, outcomeFile))
	exit, runErr := l.session.Exec(attemptCtx, command, prompt,
		io.MultiWriter(l.tap, l.events))
	timedOut := errors.Is(attemptCtx.Err(), context.DeadlineExceeded) &&
		l.ctx.Err() == nil
	cancel()
	// What the agent said last is what you most want to read.
	l.r.flush(l.parent, l.job.ID, l.events)

	text := readOutcome(filepath.Join(l.dirs.outcome, outcomeFile))
	limited, resetsAt := l.tap.Limit()
	res := l.baseResult()
	if b, ok := findLine(text, "Blocked:"); ok && !strings.EqualFold(b, "none") &&
		!leaks(b, l.secrets) {
		res.blocked = b
	}
	var reason string
	switch {
	case l.ended() != "":
		reason = l.ended()
	case timedOut:
		reason = fmt.Sprintf("Timed out after %d minutes.", l.limit/l.r.minute())
	case runErr != nil:
		reason = "The sandbox could not run the job: " + runErr.Error()
		res.blocked = noOutcomeBlock(text, reason, l.secrets, res.blocked)
	case limited && (exit != 0 || text == ""):
		// A rejection the agent worked past, to a finished outcome, is not a stop.
		reason = limitReason(resetsAt)
		res.limited, res.resetsAt = true, resetsAt
		res.outcome = strings.TrimSpace(reason + "\n\n" + text)
		wait := again(res, "usage limit reached", "", "", limitLifted)
		// With no session there is nothing to resume: the limit ends the job.
		wait.done = l.tap.Session() == ""
		wait.limited, wait.waitUntil = true, resetsAt
		return "", &wait
	case exit != 0:
		reason = fmt.Sprintf("The agent exited with status %d.", exit)
		res.blocked = noOutcomeBlock(text, reason, l.secrets, res.blocked)
	case text == "":
		reason = "The agent exited cleanly but wrote no outcome file."
		res.blocked = noOutcomeBlock(text, reason, l.secrets, res.blocked)
	default:
		return text, nil
	}
	res.outcome = strings.TrimSpace(reason + "\n\n" + text)
	end := over(res)
	return "", &end
}

// waitOutLimit keeps the VM until a usage limit lifts. The check-ins go on
// meanwhile, so the server doesn't call the job lost.
func (l *jobLoop) waitOutLimit(end time.Time) {
	l.sleep(end.Sub(l.now()))
}

// limitEnd is when to resume after a limit: the time the harness gave, within
// reason, or an hour from now when it gave none.
func (l *jobLoop) limitEnd(until time.Time) time.Time {
	now := l.now()
	if until.IsZero() {
		return now.Add(defaultLimitWait)
	}
	return minTime(maxTime(until, now), now.Add(maxLimitWait))
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// watch follows the job's open pull request until it merges, closes or fails CI.
func (l *jobLoop) watch(res result, text string) step {
	number, ok := pullNumber(res.prURL)
	if !ok {
		return over(res)
	}
	ref := PullRef{Owner: l.job.RepoOwner, Repo: l.job.RepoName, Number: number}
	deadline := l.now().Add(cmp.Or(l.r.MaxWatch, DefaultMaxWatch))
	for {
		if why := l.ended(); why != "" {
			return l.finish(res, why, text)
		}
		if st, err := l.status(ref); err != nil {
			l.r.Log.Warn("could not read the pull request", "job", l.job.ID,
				"error", scrub(err.Error(), l.token))
		} else if s, over := l.judge(res, text, st); over {
			return s
		}
		if l.now().After(deadline) {
			return l.finish(res, "The pull request was not merged within "+
				cmp.Or(l.r.MaxWatch, DefaultMaxWatch).String()+".", text)
		}
		l.sleep(cmp.Or(l.r.PollEvery, DefaultPollEvery))
	}
}

// judge is what a pull request's status means for the job; over says it does.
func (l *jobLoop) judge(res result, text string, st PullStatus) (step, bool) {
	switch {
	case st.State == PullMerged:
		res.succeeded, res.outcome = true, text
		return over(res), true
	case st.State == PullClosed:
		return l.finish(res, "The pull request was closed without merging.", text), true
	case st.CI != CIFailure || (res.head != "" && st.HeadSHA != res.head):
		return over(res), false
	}
	names := make([]string, len(st.Failed))
	var detail strings.Builder
	for i, f := range st.Failed {
		names[i] = f.Name
		detail.WriteString("\n\n" + f.Name + ":\n" + f.Excerpt)
	}
	joined := strings.Join(names, ", ")
	return again(res, "CI failed on "+short(joined), "ci:"+joined, st.HeadSHA,
		"The pull request "+res.prURL+" is open, but CI failed on "+joined+"."+
			detail.String()+"\n\nFix it, run those gates yourself where the sandbox "+
			"allows, commit on the current branch (the runner pushes it to the same "+
			"pull request), and write the outcome again. Your work continues until "+
			"the pull request merges."), true
}

// finish ends the job on a pull request that will not merge, keeping its link.
func (l *jobLoop) finish(res result, reason, text string) step {
	res.succeeded = false
	res.outcome = strings.TrimSpace(reason + "\n\n" + text)
	return over(res)
}

// status reads the pull request with a push token that is reused for a while.
func (l *jobLoop) status(ref PullRef) (PullStatus, error) {
	if l.token == "" || l.now().Sub(l.tokenAt) > tokenLife {
		token, err := l.r.pushToken(l.ctx, l.job.ID)
		if err != nil || token == "" {
			return PullStatus{}, fmt.Errorf("get a token: %w",
				cmp.Or(err, errors.New("none given")))
		}
		l.token, l.tokenAt = token, l.now()
	}
	ctx, cancel := context.WithTimeout(l.ctx, publishTimeout)
	defer cancel()
	return l.r.GitHub.PullRequestStatus(ctx, l.token, ref)
}

// note tells the board an attempt is starting: a line in the job's log and a
// comment on its task. Neither is worth failing the job over.
func (l *jobLoop) note(text string) {
	text = redact(strings.TrimSpace(text), l.events.redactions)
	l.events.note(text)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(l.parent), noteTimeout)
	defer cancel()
	if err := l.r.API.AddJobComment(ctx, l.r.Token, l.job.ID, text); err != nil {
		l.r.Log.Warn("could not comment on the task", "job", l.job.ID, "error", err)
	}
}

// short keeps a reason to one line fit for a comment.
func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxWhy {
		s = strings.ToValidUTF8(s[:maxWhy], "") + "..."
	}
	return s
}
