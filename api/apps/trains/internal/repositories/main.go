// Package repositories is the trains app's DB access layer.
package repositories

import "tools.xdoubleu.com/internal/database/postgres"

type Repositories struct {
	SavedCommutes *SavedCommutesRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		SavedCommutes: &SavedCommutesRepository{db: db},
	}
}
