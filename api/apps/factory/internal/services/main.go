// Package services holds the factory's task board rules.
package services

import (
	"time"

	"tools.xdoubleu.com/apps/factory/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth    auth.Service
	Tasks   *TaskService
	Repos   *RepoService
	Runners *RunnerService
	Jobs    *JobService
}

func New(
	repos *repositories.Repositories,
	authService auth.Service,
	accounts JobAccounts,
) *Services {
	return &Services{
		Auth:    authService,
		Tasks:   &TaskService{repo: repos.Tasks},
		Repos:   &RepoService{repo: repos.Repos},
		Runners: &RunnerService{repo: repos.Runners, now: time.Now},
		Jobs: &JobService{
			jobs: repos.Jobs, tasks: repos.Tasks, repos: repos.Repos,
			events: repos.Events, accounts: accounts, now: time.Now,
		},
	}
}
