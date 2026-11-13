package repositories

import "tools.xdoubleu.com/internal/kit/database/postgres"

type Repositories struct {
	Shows *ShowsRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Shows: &ShowsRepository{db: db},
	}
}
