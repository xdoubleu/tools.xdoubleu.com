// Package dashboard owns public sharing of the Games and Reading dashboards
// via opaque tokens. It has no schema and reaches other apps only through
// their exported methods (docs/adr-0007-dashboard-app-owns-public-sharing.md).
package dashboard

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

type Dashboard struct {
	app.Base
	games         *games.Games
	books         *books.Books
	feeds         *feeds.Feeds
	profileShares *sharedrepos.ProfileSharesRepository
}

// New constructs the dashboard app. gamesApp/booksApp/feedsApp must already
// be constructed (registration order in cmd/api/apps.go).
func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	gamesApp *games.Games,
	booksApp *books.Books,
	feedsApp *feeds.Feeds,
) *Dashboard {
	return &Dashboard{
		Base:          app.NewBase(context.Background(), authService, logger, cfg),
		games:         gamesApp,
		books:         booksApp,
		feeds:         feedsApp,
		profileShares: sharedrepos.NewProfileSharesRepository(db),
	}
}

func (a *Dashboard) ApplyMigrations(_ context.Context, _ *pgxpool.Pool) error {
	return nil
}

func (a *Dashboard) Start() error {
	return nil
}

func (a *Dashboard) GetName() string {
	return "dashboard"
}

func (a *Dashboard) GetDisplayName() string {
	return "Dashboards"
}
