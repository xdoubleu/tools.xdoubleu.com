package github

import (
	"time"
)

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
//
//nolint:gochecknoglobals // static lookup table
var failingConclusions = map[string]bool{
	"failure":         true,
	"timed_out":       true,
	"cancelled":       true,
	"action_required": true,
}
