package repositories

import "github.com/xdoubleu/essentia/v4/pkg/database/postgres"

type Repositories struct {
	Plans *PlansRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Plans: &PlansRepository{db: db},
	}
}
