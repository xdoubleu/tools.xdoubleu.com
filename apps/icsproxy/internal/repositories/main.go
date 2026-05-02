package repositories

import (
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
)

type Repositories struct {
	Calendar *CalendarRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Calendar: &CalendarRepository{db: db},
	}
}
