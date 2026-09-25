package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"tools.xdoubleu.com/internal/database/postgres"
)

// Querier is the subset shared by postgres.DB and pgx.Tx; write methods take
// one (nil = the repository's connection).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type Repositories struct {
	Steam        *SteamRepository
	Progress     *ProgressRepository
	Integrations *IntegrationsRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Steam:        &SteamRepository{db: db},
		Progress:     &ProgressRepository{db: db},
		Integrations: &IntegrationsRepository{db: db},
	}
}
