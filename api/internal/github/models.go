package github

import (
	"encoding/json"
	"time"
)

// Issue is the normalised representation of a GitHub open issue.
type Issue struct {
	Number    int64
	Title     string
	URL       string
	State     string
	CreatedAt time.Time
	Labels    []string
}

// issueWire is the subset of the GitHub issues API payload that is decoded.
// GitHub's /issues endpoint returns pull requests too; the pullRequest field
// is present only on PRs and is used to filter them out.
type issueWire struct {
	Number      int64            `json:"number"`
	Title       string           `json:"title"`
	HTMLURL     string           `json:"html_url"`
	State       string           `json:"state"`
	CreatedAt   time.Time        `json:"created_at"`
	Labels      []labelWire      `json:"labels"`
	PullRequest *json.RawMessage `json:"pull_request"`
}

type labelWire struct {
	Name string `json:"name"`
}

// toIssue maps a wire issue to the exported Issue.
func (w issueWire) toIssue() Issue {
	labels := make([]string, 0, len(w.Labels))
	for _, l := range w.Labels {
		labels = append(labels, l.Name)
	}
	return Issue{
		Number:    w.Number,
		Title:     w.Title,
		URL:       w.HTMLURL,
		State:     w.State,
		CreatedAt: w.CreatedAt,
		Labels:    labels,
	}
}

// FailingCheck is a single non-passing CI check run on a pull request's head
// commit.
type FailingCheck struct {
	Name       string
	Conclusion string
	URL        string
}

// PullRequest is an open pull request with at least one failing check.
type PullRequest struct {
	Number        int64
	Title         string
	URL           string
	Author        string
	UpdatedAt     time.Time
	FailingChecks []FailingCheck
}

// prWire is the subset of the GitHub pulls API payload that is decoded.
type prWire struct {
	Number    int64     `json:"number"`
	Title     string    `json:"title"`
	HTMLURL   string    `json:"html_url"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// checkRunsWire is the GitHub "list check runs for a ref" response.
type checkRunsWire struct {
	CheckRuns []checkRunWire `json:"check_runs"`
}

type checkRunWire struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
}

// failingConclusions are the check-run conclusions treated as "failing" for
// the observability dashboard. In-progress/queued runs (empty conclusion) are
// not failing yet, just not done.
var failingConclusions = map[string]bool{ //nolint:gochecknoglobals // static lookup table
	"failure":         true,
	"timed_out":       true,
	"cancelled":       true,
	"action_required": true,
}
