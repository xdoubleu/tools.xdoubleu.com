package repositories

import (
	"tools.xdoubleu.com/internal/database/postgres"
)

type Repositories struct {
	Feeds       *FeedsRepository
	Items       *ItemsRepository
	FilterRules *FilterRulesRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Feeds:       &FeedsRepository{db: db},
		Items:       &ItemsRepository{db: db},
		FilterRules: &FilterRulesRepository{db: db},
	}
}
