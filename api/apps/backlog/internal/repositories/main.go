package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
)

// Querier is the subset of operations shared by postgres.DB and pgx.Tx. Write
// methods take a Querier so they can run either directly on the pool or inside a
// transaction; pass nil to default to the repository's own connection.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type Repositories struct {
	Books        *BooksRepository
	BookFiles    *BookFilesRepository
	ReadingState *BookReadingStateRepository
	Steam        *SteamRepository
	Progress     *ProgressRepository
	Integrations *IntegrationsRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Books:        &BooksRepository{db: db},
		BookFiles:    &BookFilesRepository{db: db},
		ReadingState: &BookReadingStateRepository{db: db},
		Steam:        &SteamRepository{db: db},
		Progress:     &ProgressRepository{db: db},
		Integrations: &IntegrationsRepository{db: db},
	}
}
