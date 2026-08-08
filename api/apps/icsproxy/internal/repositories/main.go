package repositories

import (
	"tools.xdoubleu.com/internal/database/postgres"
)

type Repositories struct {
	Calendar *CalendarRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Calendar: &CalendarRepository{db: db},
	}
}
