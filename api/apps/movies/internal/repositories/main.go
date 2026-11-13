package repositories

import "tools.xdoubleu.com/internal/kit/database/postgres"

type Repositories struct {
	Movies *MoviesRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Movies: &MoviesRepository{db: db},
	}
}
