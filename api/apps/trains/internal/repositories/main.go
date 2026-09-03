// Package repositories is the trains app's DB access layer over the trains
// schema.
package repositories

import "tools.xdoubleu.com/internal/database/postgres"

type Repositories struct {
	Feed *FeedRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Feed: &FeedRepository{db: db},
	}
}
