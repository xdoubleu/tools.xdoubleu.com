package repositories

import "tools.xdoubleu.com/internal/database/postgres"

type Repositories struct {
	Tasks   *TasksRepository
	Repos   *ReposRepository
	Runners *RunnersRepository
	Jobs    *JobsRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Tasks:   &TasksRepository{db: db},
		Repos:   &ReposRepository{db: db},
		Runners: &RunnersRepository{db: db},
		Jobs:    &JobsRepository{db: db},
	}
}
