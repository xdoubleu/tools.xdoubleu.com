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
	HeadSHA       string
	Labels        []string
	FailingChecks []FailingCheck
}

// HasLabel reports whether the pull request carries the given label (e.g.
// "dependencies", set by Renovate on every PR it opens — see
// renovate.json5).
func (pr PullRequest) HasLabel(name string) bool {
	for _, l := range pr.Labels {
		if l == name {
			return true
		}
	}
	return false
}

// SecurityAlert is a single open Dependabot alert on the repo's dependencies.
type SecurityAlert struct {
	Number      int64
	PackageName string
	Ecosystem   string
	Severity    string
	Summary     string
	URL         string
	CreatedAt   time.Time
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
	Labels []labelWire `json:"labels"`
}

// labelWire is a single entry in a pull request's "labels" array.
type labelWire struct {
	Name string `json:"name"`
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

// securityAlertWire is the subset of the GitHub Dependabot alerts API
// payload that is decoded.
type securityAlertWire struct {
	Number     int64     `json:"number"`
	HTMLURL    string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	Dependency struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
	} `json:"dependency"`
	SecurityAdvisory struct {
		Summary string `json:"summary"`
	} `json:"security_advisory"`
	SecurityVulnerability struct {
		Severity string `json:"severity"`
	} `json:"security_vulnerability"`
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
