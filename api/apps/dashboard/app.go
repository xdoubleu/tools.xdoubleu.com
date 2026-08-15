// Package dashboard centralizes public dashboard sharing (issue #737):
// hardcoded Games and Reading (books+feeds) dashboards, each shareable via
// an opaque token. It owns no schema of its own — no config/widget
// framework exists yet, so there is nothing to persist beyond the tokens
// already stored in global.profile_shares — and reaches games/books/feeds
// only through the exported methods those apps expose (BuildSharedSteam,
// BuildSharedLibrary, BuildSummary, ...), never their internal packages or
// schemas directly.
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
// be constructed — dashboard registers after them in cmd/api/apps.go so
// these live references exist by the time it's built.
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
