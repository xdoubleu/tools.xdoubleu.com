package github

import (
	"slices"
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

// DependenciesLabel is the label Renovate sets on every PR it opens (see
// renovate.json5).
const DependenciesLabel = "dependencies"

// HasLabel reports whether the pull request carries the given label (e.g.
// DependenciesLabel).
func (pr PullRequest) HasLabel(name string) bool {
	return slices.Contains(pr.Labels, name)
}

// SecurityAlertType distinguishes which GitHub alert source a SecurityAlert
// came from — the three fields below it are populated only for the
// corresponding type.
type SecurityAlertType string

const (
	SecurityAlertTypeDependabot     SecurityAlertType = "dependabot"
	SecurityAlertTypeCodeScanning   SecurityAlertType = "code_scanning"
	SecurityAlertTypeSecretScanning SecurityAlertType = "secret_scanning"
)

// SecurityAlert is a single open Dependabot, code-scanning, or
// secret-scanning alert on the repo. PackageName/Ecosystem are populated only
// for Type == SecurityAlertTypeDependabot; RuleID/FilePath/Line only for
// SecurityAlertTypeCodeScanning; SecretTypeDisplayName only for
// SecurityAlertTypeSecretScanning.
type SecurityAlert struct {
	Type                  SecurityAlertType
	Number                int64
	PackageName           string
	Ecosystem             string
	Severity              string
	Summary               string
	URL                   string
	CreatedAt             time.Time
	RuleID                string
	FilePath              string
	Line                  int64
	SecretTypeDisplayName string
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

// WorkflowRun is a single GitHub Actions workflow run, either from a pull
// request or a push to the default branch. DurationMs is only meaningful
// once Status is "completed" — zero for runs still in progress.
type WorkflowRun struct {
	ID         int64
	Name       string
	Event      string // "pull_request" | "push"
	Branch     string
	Status     string
	Conclusion string
	URL        string
	StartedAt  time.Time
	DurationMs int64
}

// workflowRunsWire is the GitHub "list workflow runs for a repository"
// response.
type workflowRunsWire struct {
	WorkflowRuns []workflowRunWire `json:"workflow_runs"`
}

type workflowRunWire struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Event        string    `json:"event"`
	HeadBranch   string    `json:"head_branch"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	HTMLURL      string    `json:"html_url"`
	RunStartedAt time.Time `json:"run_started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// WorkflowJob is a single job within a GitHub Actions workflow run —
// DurationMs is only meaningful once Status is "completed".
type WorkflowJob struct {
	Name        string
	Status      string
	Conclusion  string
	StartedAt   time.Time
	CompletedAt time.Time
	DurationMs  int64
}

// workflowJobsWire is the GitHub "list jobs for a workflow run" response.
type workflowJobsWire struct {
	Jobs []workflowJobWire `json:"jobs"`
}

type workflowJobWire struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
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

// codeScanningAlertWire is the subset of the GitHub code scanning alerts API
// payload that is decoded.
type codeScanningAlertWire struct {
	Number    int64     `json:"number"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	Rule      struct {
		ID                    string `json:"id"`
		Description           string `json:"description"`
		SecuritySeverityLevel string `json:"security_severity_level"`
	} `json:"rule"`
	MostRecentInstance struct {
		Location struct {
			Path      string `json:"path"`
			StartLine int64  `json:"start_line"`
		} `json:"location"`
	} `json:"most_recent_instance"`
}

// secretScanningAlertWire is the subset of the GitHub secret scanning alerts
// API payload that is decoded.
type secretScanningAlertWire struct {
	Number                int64     `json:"number"`
	HTMLURL               string    `json:"html_url"`
	CreatedAt             time.Time `json:"created_at"`
	SecretTypeDisplayName string    `json:"secret_type_display_name"`
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
