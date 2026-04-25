package repositories

import (
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
)

type Repositories struct {
	Books        *BooksRepository
	Steam        *SteamRepository
	Progress     *ProgressRepository
	Integrations *IntegrationsRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Books:        &BooksRepository{db: db},
		Steam:        &SteamRepository{db: db},
		Progress:     &ProgressRepository{db: db},
		Integrations: &IntegrationsRepository{db: db},
	}
}
