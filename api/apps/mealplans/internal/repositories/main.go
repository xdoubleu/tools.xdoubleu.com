package repositories

import "tools.xdoubleu.com/internal/database/postgres"

type Repositories struct {
	Plans *PlansRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Plans: &PlansRepository{db: db},
	}
}
