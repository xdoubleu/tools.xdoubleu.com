package factory

import (
	"context"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/repositories"
	"tools.xdoubleu.com/internal/platform/reposignals"
)

// RepoSignals lists the registered repos' monitoring settings.
type RepoSignals struct {
	repos *repositories.ReposRepository
}

var _ reposignals.Source = (*RepoSignals)(nil)

// NewRepoSignals reads the repos table through db.
func NewRepoSignals(db postgres.DB) *RepoSignals {
	return &RepoSignals{repos: repositories.New(db).Repos}
}

// MonitoredRepos returns every registered repo.
func (s *RepoSignals) MonitoredRepos(ctx context.Context) ([]reposignals.Repo, error) {
	repos, err := s.repos.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]reposignals.Repo, len(repos))
	for i, r := range repos {
		out[i] = reposignals.Repo{
			Slug:              r.Owner + "/" + r.Name,
			SentryProject:     r.SentryProject,
			RedPRLabel:        r.RedPRLabel,
			RedPRBranchPrefix: r.RedPRBranchPrefix,
		}
	}
	return out, nil
}
