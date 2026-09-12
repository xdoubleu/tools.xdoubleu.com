// Package learningpaths implements agent-authorable learning curricula: a
// LearningPath (goal, recurring routine) made of ordered Modules, each with
// ordered Items a user checks off as they progress, plus a freeform
// resources list. This slice (#1472) is the core app — proto, schema,
// ConnectRPC CRUD, and a minimal web UI. MCP tools (#1473), books/feeds
// resource linking (#1474), and Todoist integration (#1475) are later,
// independent slices stacked on top of this one.
package learningpaths

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/todoist"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type LearningPaths struct {
	app.Base
	services *services.Services
}

// sealer encrypts/decrypts the per-user Todoist OAuth tokens stored in
// learningpaths.oauth_connections (issue #1475) — a separate table from
// global.oauth_connections, reusing the same api/internal/crypto.Sealer the
// rest of the app's OAuth-connected integrations use.
//
// booksApp/feedsApp must already be constructed — learningpaths registers
// after them in cmd/api/apps.go so these live references exist by the time
// it's built. They are passed straight through to the service layer, which
// calls only their exported methods (Books.GetLibraryBookByID,
// Feeds.GetItemByID) to resolve a resource linked to a books/feeds entry —
// the same dashboard-style cross-app pattern as api/apps/dashboard
// (docs/adr-0007-dashboard-app-owns-public-sharing.md), never their
// internal/ packages or schemas directly.
func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sealer *crypto.Sealer,
	booksApp *books.Books,
	feedsApp *feeds.Feeds,
) *LearningPaths {
	//nolint:exhaustruct //services initialised below
	a := &LearningPaths{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
		),
	}
	todoistConf := todoist.OAuthConfig(
		cfg.TodoistOAuthClientID, cfg.TodoistOAuthClientSecret, cfg.APIURL,
	)
	a.services = services.New(
		a.Logger,
		repositories.New(db, sealer),
		authService,
		todoistConf,
		booksApp,
		feedsApp,
	)

	return a
}

func (a *LearningPaths) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *LearningPaths) Start() error {
	return nil
}

func (a *LearningPaths) GetName() string {
	return "learningpaths"
}

func (a *LearningPaths) GetDisplayName() string {
	return "Learning Paths"
}

// TodoistServiceForTest exposes the Todoist service so an external test
// (package learningpaths_test) can stub its OAuth2 config via
// services.TodoistService.SetOAuthConfigForTest — test-only, mirroring how
// cmd/api's tests reach into its Application for the equivalent admin OAuth
// stubbing.
func (a *LearningPaths) TodoistServiceForTest() *services.TodoistService {
	return a.services.Todoist
}
