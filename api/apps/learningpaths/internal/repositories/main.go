package repositories

import (
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
)

type Repositories struct {
	LearningPaths    *LearningPathsRepository
	OAuthConnections *OAuthConnectionsRepository
}

// New wires this app's repositories. sealer encrypts Todoist tokens; when nil
// (ENCRYPTION_KEY unset), token reads and writes return
// ErrEncryptionNotConfigured.
func New(db postgres.DB, sealer *crypto.Sealer) *Repositories {
	return &Repositories{
		LearningPaths:    &LearningPathsRepository{db: db},
		OAuthConnections: NewOAuthConnectionsRepository(db, sealer),
	}
}
