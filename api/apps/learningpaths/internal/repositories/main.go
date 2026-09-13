package repositories

import (
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
)

type Repositories struct {
	LearningPaths    *LearningPathsRepository
	OAuthConnections *OAuthConnectionsRepository
}

// New wires this app's repositories. sealer encrypts/decrypts stored Todoist
// tokens (issue #1475) — nil in tests that never exercise OAuthConnections.
func New(db postgres.DB, sealer *crypto.Sealer) *Repositories {
	return &Repositories{
		LearningPaths:    &LearningPathsRepository{db: db},
		OAuthConnections: NewOAuthConnectionsRepository(db, sealer),
	}
}
