package github

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// FailingCheck is a non-passing check run on a PR's head commit.
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

// DependenciesLabel is the label Renovate sets on its PRs.
const DependenciesLabel = "dependencies"

// HasLabel reports whether the pull request carries name.
func (pr PullRequest) HasLabel(name string) bool {
	return slices.Contains(pr.Labels, name)
}

// SecurityAlertType is the alert source; it selects which SecurityAlert fields
// are populated.
type SecurityAlertType string

const (
	SecurityAlertTypeDependabot     SecurityAlertType = "dependabot"
	SecurityAlertTypeCodeScanning   SecurityAlertType = "code_scanning"
	SecurityAlertTypeSecretScanning SecurityAlertType = "secret_scanning"
)

// SecurityAlert is one open alert. PackageName/Ecosystem are set for
// Dependabot, RuleID/FilePath/Line for code scanning, SecretTypeDisplayName for
// secret scanning.
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

// ErrInvalidDismissReason means reason or alertType isn't one GitHub accepts.
var ErrInvalidDismissReason = errors.New("github: invalid dismiss reason")

// The dismiss reasons GitHub accepts per alert type, validated client-side.
//
//nolint:gochecknoglobals // static lookup tables
var dependabotDismissReasons = map[string]bool{
	"fix_started":    true,
	"inaccurate":     true,
	"no_bandwidth":   true,
	"not_used":       true,
	"tolerable_risk": true,
}

//nolint:gochecknoglobals // static lookup tables
var codeScanningDismissReasons = map[string]bool{
	"false positive": true,
	"won't fix":      true,
	"used in tests":  true,
}

//nolint:gochecknoglobals // static lookup tables
var secretScanningDismissReasons = map[string]bool{
	"false_positive":  true,
	"wont_fix":        true,
	"revoked":         true,
	"used_in_tests":   true,
	"pattern_deleted": true,
}

func dismissRequest(
	repo string, alertType SecurityAlertType, alertNumber int64, reason string,
) (string, string, error) {
	switch alertType {
	case SecurityAlertTypeDependabot:
		if !dependabotDismissReasons[reason] {
			return "", "", fmt.Errorf("%w: %q", ErrInvalidDismissReason, reason)
		}
		endpoint := fmt.Sprintf(
			"%s/repos/%s/dependabot/alerts/%d", baseURL, repo, alertNumber,
		)
		body := fmt.Sprintf(`{"state":"dismissed","dismissed_reason":%q}`, reason)
		return endpoint, body, nil
	case SecurityAlertTypeCodeScanning:
		if !codeScanningDismissReasons[reason] {
			return "", "", fmt.Errorf("%w: %q", ErrInvalidDismissReason, reason)
		}
		endpoint := fmt.Sprintf(
			"%s/repos/%s/code-scanning/alerts/%d", baseURL, repo, alertNumber,
		)
		body := fmt.Sprintf(`{"state":"dismissed","dismissed_reason":%q}`, reason)
		return endpoint, body, nil
	case SecurityAlertTypeSecretScanning:
		if !secretScanningDismissReasons[reason] {
			return "", "", fmt.Errorf("%w: %q", ErrInvalidDismissReason, reason)
		}
		endpoint := fmt.Sprintf(
			"%s/repos/%s/secret-scanning/alerts/%d", baseURL, repo, alertNumber,
		)
		body := fmt.Sprintf(`{"state":"resolved","resolution":%q}`, reason)
		return endpoint, body, nil
	default:
		return "", "", fmt.Errorf(
			"%w: unknown alert type %q", ErrInvalidDismissReason, alertType,
		)
	}
}

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

type labelWire struct {
	Name string `json:"name"`
}

type checkRunsWire struct {
	CheckRuns []checkRunWire `json:"check_runs"`
}

type checkRunWire struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
}

// WorkflowRun is a GitHub Actions run; DurationMs is zero until completed.
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

// WorkflowJob is a job in a workflow run; DurationMs is zero until completed.
type WorkflowJob struct {
	Name        string
	Status      string
	Conclusion  string
	StartedAt   time.Time
	CompletedAt time.Time
	DurationMs  int64
}

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

type secretScanningAlertWire struct {
	Number                int64     `json:"number"`
	HTMLURL               string    `json:"html_url"`
	CreatedAt             time.Time `json:"created_at"`
	SecretTypeDisplayName string    `json:"secret_type_display_name"`
}

// Empty conclusion means still running, not failing.
//
//nolint:gochecknoglobals // static lookup table
var failingConclusions = map[string]bool{
	"failure":         true,
	"timed_out":       true,
	"cancelled":       true,
	"action_required": true,
}
