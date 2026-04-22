package repositories

import (
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
)

type Repositories struct {
	Goodreads    *GoodreadsRepository
	Steam        *SteamRepository
	Progress     *ProgressRepository
	Integrations *IntegrationsRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Goodreads:    &GoodreadsRepository{db: db},
		Steam:        &SteamRepository{db: db},
		Progress:     &ProgressRepository{db: db},
		Integrations: &IntegrationsRepository{db: db},
	}
}
