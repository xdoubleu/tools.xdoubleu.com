// Package services holds the factory's task board rules.
package services

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Services struct {
	Tasks   *TaskService
	Repos   *RepoService
	Runners *RunnerService
	Jobs    *JobService
	Board   *BoardService
	Chats   *ChatService
}

func New(
	stores Stores,
	accounts JobAccounts,
	github GitHub,
	notifier Notifier,
	webURL, release string,
) *Services {
	say := notices{out: notifier, webURL: webURL}
	tasks := &TaskService{repo: stores.Tasks}
	jobs := &JobService{
		jobs: stores.Jobs, tasks: stores.Tasks, repos: stores.Repos,
		events: stores.Events, accounts: accounts, github: github, release: release,
		deploys: newDeployCache(), now: time.Now,
		say: say, runners: stores.Runners, settings: stores.Settings,
		linked:      stores.Linked,
		routines:    stores.Routines,
		chats:       stores.Chats,
		imageDigest: nil,
		offlineMu:   sync.Mutex{}, alertedOffline: uuid.Nil, alertedStalled: uuid.Nil,
	}
	return &Services{
		Tasks:   tasks,
		Repos:   &RepoService{repo: stores.Repos},
		Runners: &RunnerService{repo: stores.Runners, now: time.Now, say: say},
		Jobs:    jobs,
		Board:   &BoardService{tasks: tasks, jobs: jobs},
		Chats: &ChatService{
			chats: stores.Chats, repos: stores.Repos, tasks: stores.Tasks,
			runners: stores.Runners, now: time.Now, askPoll: askPollEvery,
		},
	}
}
