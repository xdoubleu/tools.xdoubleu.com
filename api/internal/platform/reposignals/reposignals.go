// Package reposignals carries each registered repo's monitoring settings from
// the factory's repo config to the observability signals, without either
// importing the other.
package reposignals

import (
	"context"
	"strings"
)

// Repo is one repo's monitoring scope.
type Repo struct {
	// Slug is "owner/name".
	Slug string
	// SentryProject is the repo's Sentry project slug; empty for none.
	SentryProject string
	// RedPRLabel and RedPRBranchPrefix select the PRs that count as red; a PR
	// matching either is in scope. Empty turns that rule off.
	RedPRLabel        string
	RedPRBranchPrefix string
}

// Source lists the repos to monitor.
type Source interface {
	MonitoredRepos(ctx context.Context) ([]Repo, error)
}

// RepoForSentryProject returns the slug of the repo that owns project, or "".
func RepoForSentryProject(repos []Repo, project string) string {
	for _, r := range repos {
		if r.SentryProject != "" && strings.EqualFold(r.SentryProject, project) {
			return r.Slug
		}
	}
	return ""
}

// Find returns the repo with the given slug, case-insensitively.
func Find(repos []Repo, slug string) (Repo, bool) {
	for _, r := range repos {
		if strings.EqualFold(r.Slug, slug) {
			return r, true
		}
	}
	return Repo{Slug: "", SentryProject: "", RedPRLabel: "", RedPRBranchPrefix: ""}, false
}
