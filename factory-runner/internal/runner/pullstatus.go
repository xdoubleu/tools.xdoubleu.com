package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	// RequiredCheck is the CI check a pull request needs to merge.
	RequiredCheck = "ci-pass"

	// Pull request and CI states as PullRequestStatus reports them.
	PullOpen    = "open"
	PullMerged  = "merged"
	PullClosed  = "closed"
	CIPending   = "pending"
	CISuccess   = "success"
	CIFailure   = "failure"
	maxFailures = 8
	// maxAnnotations is how many annotations of a failed check are read.
	maxAnnotations = 10
	maxExcerpt     = 800
	// maxLogTail is how much of an Actions job log is kept, ending at its first error.
	maxLogTail = 4096
	// maxLogBody is how much of a job log is read.
	maxLogBody = 16 << 20
)

var (
	jobURLPattern = regexp.MustCompile(`/actions/runs/[0-9]+/job/([0-9]+)(?:[?#].*)?$`)
	logPrefix     = regexp.MustCompile(`(?m)^(?:\x{FEFF})?\d{4}-\d\d-\d\dT[0-9:.]+Z `)
	ansiPattern   = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
)

var pullURLPattern = regexp.MustCompile(`/pull/([0-9]{1,9})/?$`)

// pullNumber is the number in a pull request's address, if it has one.
func pullNumber(prURL string) (int, bool) {
	m := pullURLPattern.FindStringSubmatch(prURL)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// PullRef names a pull request.
type PullRef struct {
	Owner, Repo string
	Number      int
}

// CheckFailure is one failed CI check and what GitHub says about it.
type CheckFailure struct {
	Name    string
	Excerpt string
}

// PullStatus is where a job's pull request stands.
type PullStatus struct {
	// State is PullOpen, PullMerged or PullClosed.
	State string
	// HeadSHA is the commit the pull request is at.
	HeadSHA string
	// CI is CIPending until a check has failed or RequiredCheck has finished,
	// then CISuccess or CIFailure.
	CI string
	// Failed lists the failed checks of a CIFailure.
	Failed []CheckFailure
	// Conflict is set when the pull request conflicts with its base. While GitHub
	// is still computing that, it stays false.
	Conflict bool
}

type checkRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	DetailsURL string `json:"details_url"`
	Output     struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
	} `json:"output"`
}

// failing is a conclusion that makes a check red.
func (c checkRun) failing() bool {
	return c.Status == "completed" && slices.Contains(
		[]string{"failure", "timed_out", "cancelled", "action_required", "startup_failure"},
		c.Conclusion)
}

// PullRequestStatus implements GitHubAPI.
func (c *GitHubClient) PullRequestStatus(
	ctx context.Context,
	token string,
	ref PullRef,
) (PullStatus, error) {
	repo := "/repos/" + url.PathEscape(ref.Owner) + "/" + url.PathEscape(ref.Repo)
	var pull struct {
		State  string `json:"state"`
		Merged bool   `json:"merged"`
		// Mergeable is null while GitHub computes it.
		Mergeable      *bool  `json:"mergeable"`
		MergeableState string `json:"mergeable_state"`
		Head           struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	path := repo + "/pulls/" + strconv.Itoa(ref.Number)
	if err := c.do(ctx, http.MethodGet, path, token, nil, &pull); err != nil {
		return PullStatus{}, fmt.Errorf("read the pull request: %w", err)
	}
	status := PullStatus{
		State: PullOpen, HeadSHA: pull.Head.SHA, CI: CIPending, Failed: nil,
		Conflict: (pull.Mergeable != nil && !*pull.Mergeable) ||
			pull.MergeableState == "dirty",
	}
	switch {
	case pull.Merged:
		status.State = PullMerged
		return status, nil
	case pull.State == "closed":
		status.State = PullClosed
		return status, nil
	case pull.Head.SHA == "":
		return status, nil
	}
	var runs struct {
		CheckRuns []checkRun `json:"check_runs"`
	}
	path = repo + "/commits/" + url.PathEscape(pull.Head.SHA) + "/check-runs?per_page=100"
	if err := c.do(ctx, http.MethodGet, path, token, nil, &runs); err != nil {
		return PullStatus{}, fmt.Errorf("read the checks: %w", err)
	}
	c.judge(ctx, repo, token, runs.CheckRuns, &status)
	return status, nil
}

// judge sets the CI state and lists what failed: a red check fails it at once while
// ci-pass is unfinished, then ci-pass decides.
func (c *GitHubClient) judge(
	ctx context.Context,
	repo, token string,
	runs []checkRun,
	status *PullStatus,
) {
	var gate *checkRun
	var red []checkRun
	for i, r := range runs {
		switch {
		case r.Name == RequiredCheck:
			gate = &runs[i]
		case r.failing():
			red = append(red, r)
		}
	}
	gateDone := gate != nil && gate.Status == "completed"
	if !gateDone {
		// A cancelled run may be replaced by a newer one, so it is no early verdict.
		red = slices.DeleteFunc(red, func(r checkRun) bool {
			return r.Conclusion == "cancelled"
		})
	}
	switch {
	case gateDone && !gate.failing():
		status.CI = CISuccess
		return
	case len(red) == 0 && !gateDone:
		return
	}
	// One red check is enough: the verdict does not wait for the rest to finish.
	status.CI = CIFailure
	if len(red) == 0 {
		// ci-pass only says that something failed; with no other red check, it is
		// all there is to name.
		red = []checkRun{*gate}
	}
	for _, r := range red[:min(len(red), maxFailures)] {
		status.Failed = append(status.Failed, CheckFailure{
			Name: r.Name, Excerpt: c.excerpt(ctx, repo, token, r),
		})
	}
}

// excerpt is what GitHub says about a failed check: its output and its failure
// annotations (file, line and message), which is where compilers and linters show up.
func (c *GitHubClient) excerpt(
	ctx context.Context,
	repo, token string,
	r checkRun,
) string {
	var parts []string
	if t := strings.TrimSpace(r.Output.Title + " " + r.Output.Summary); t != "" {
		parts = append(parts, t)
	}
	var notes []struct {
		Path    string `json:"path"`
		Line    int    `json:"start_line"`
		Level   string `json:"annotation_level"`
		Message string `json:"message"`
	}
	path := repo + "/check-runs/" + strconv.FormatInt(r.ID, 10) +
		"/annotations?per_page=" + strconv.Itoa(maxAnnotations)
	// Annotations are a bonus: a failed read leaves the rest.
	_ = c.do(ctx, http.MethodGet, path, token, nil, &notes)
	for _, n := range notes {
		if n.Level == "failure" || n.Level == "" {
			parts = append(parts, fmt.Sprintf("%s:%d: %s", n.Path, n.Line, n.Message))
		}
	}
	text := strings.Join(parts, "\n")
	if len(text) > maxExcerpt {
		text = strings.ToValidUTF8(text[:maxExcerpt], "") + "..."
	}
	if tail := c.logTail(ctx, repo, token, r); tail != "" {
		text = strings.TrimSpace(text + "\n" + tail)
	}
	return text
}

// logTail is the end of a failed Actions job's log up to its first error, where a
// failing test's name and assertion are. It is empty for a check that is not an
// Actions job or whose log can't be read (the App may lack actions:read).
func (c *GitHubClient) logTail(
	ctx context.Context,
	repo, token string,
	r checkRun,
) string {
	m := jobURLPattern.FindStringSubmatch(r.DetailsURL)
	if m == nil {
		return ""
	}
	raw, err := c.fetch(ctx, http.MethodGet, repo+"/actions/jobs/"+m[1]+"/logs",
		token, nil, maxLogBody)
	if err != nil {
		return ""
	}
	return scrub(tailAroundError(string(raw)), token)
}

// tailAroundError cleans a job log of timestamps and colour codes and keeps the
// last maxLogTail bytes up to and including its first ##[error] line (or the end
// of the log without one).
func tailAroundError(log string) string {
	log = ansiPattern.ReplaceAllString(logPrefix.ReplaceAllString(log, ""), "")
	log = strings.TrimRight(log, "\r\n")
	if i := strings.Index(log, "##[error]"); i >= 0 {
		if end := strings.IndexByte(log[i:], '\n'); end >= 0 {
			log = log[:i+end]
		}
	}
	if len(log) > maxLogTail {
		log = log[len(log)-maxLogTail:]
		if nl := strings.IndexByte(log, '\n'); nl >= 0 {
			log = log[nl+1:]
		}
		log = strings.ToValidUTF8(log, "")
	}
	return strings.TrimSpace(log)
}
