// Package services holds the factory's task board rules.
package services

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/factory/internal/repositories"
)

type Services struct {
	Auth    auth.Service
	Tasks   *TaskService
	Repos   *RepoService
	Runners *RunnerService
	Jobs    *JobService
	Board   *BoardService
}

func New(
	repos *repositories.Repositories,
	authService auth.Service,
	accounts JobAccounts,
	github GitHub,
	notifier Notifier,
	webURL, release string,
) *Services {
	say := notices{out: notifier, webURL: webURL}
	tasks := &TaskService{repo: repos.Tasks}
	jobs := &JobService{
		jobs: repos.Jobs, tasks: repos.Tasks, repos: repos.Repos,
		events: repos.Events, accounts: accounts, github: github, release: release,
		deploys: newDeployCache(), now: time.Now,
		say: say, runners: repos.Runners, settings: repos.Settings,
		linked:      repos.Linked,
		routines:    repos.Routines,
		imageDigest: nil,
		offlineMu:   sync.Mutex{}, alertedOffline: uuid.Nil, alertedStalled: uuid.Nil,
	}
	return &Services{
		Auth:    authService,
		Tasks:   tasks,
		Repos:   &RepoService{repo: repos.Repos},
		Runners: &RunnerService{repo: repos.Runners, now: time.Now, say: say},
		Jobs:    jobs,
		Board:   &BoardService{tasks: tasks, jobs: jobs},
	}
}
