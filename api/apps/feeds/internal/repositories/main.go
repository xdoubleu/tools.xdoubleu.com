package repositories

import (
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
)

type Repositories struct {
	Feeds *FeedsRepository
	Items *ItemsRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Feeds: &FeedsRepository{db: db},
		Items: &ItemsRepository{db: db},
	}
}
