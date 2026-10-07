package repositories

import "tools.xdoubleu.com/internal/database/postgres"

type Repositories struct {
	Movies *MoviesRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Movies: &MoviesRepository{db: db},
	}
}
