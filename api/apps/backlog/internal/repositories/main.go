package repositories

import (
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
)

type Repositories struct {
	Books        *BooksRepository
	Steam        *SteamRepository
	Progress     *ProgressRepository
	Integrations *IntegrationsRepository
}

func New(db postgres.DB, encryptionKey []byte) *Repositories {
	return &Repositories{
		Books:        &BooksRepository{db: db},
		Steam:        &SteamRepository{db: db},
		Progress:     &ProgressRepository{db: db},
		Integrations: &IntegrationsRepository{db: db, encryptionKey: encryptionKey},
	}
}
