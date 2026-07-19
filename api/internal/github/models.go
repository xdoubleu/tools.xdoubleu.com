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
