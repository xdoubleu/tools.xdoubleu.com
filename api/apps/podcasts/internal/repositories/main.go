package repositories

import "tools.xdoubleu.com/internal/database/postgres"

type Repositories struct {
	Shows    *ShowsRepository
	Episodes *EpisodesRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Shows:    &ShowsRepository{db: db},
		Episodes: &EpisodesRepository{db: db},
	}
}
