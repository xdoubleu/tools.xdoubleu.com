// Package trains plans journeys and shows live train state through the iRail
// API.
package trains

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	trainshttp "tools.xdoubleu.com/apps/trains/http"
	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/auth"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Trains struct {
	app.Base
	db postgres.DB
	// Exported so integration tests can drive the real services.
	Services     *services.Services
	Repositories *repositories.Repositories
	journeyWS    *trainshttp.JourneyWS
}

func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Trains {
	// iRail asks for a User-Agent that identifies the app and how to reach it.
	userAgent := fmt.Sprintf("tools.xdoubleu.com (%s)", cfg.WebURL)
	return NewInner(
		ctx,
		authService, logger, cfg, db,
		irail.New(logger, irail.DefaultBaseURL, userAgent),
	)
}

// NewInner lets tests inject a fake irail.Client.
func NewInner(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	client irail.Client,
) *Trains {
	//nolint:exhaustruct //Services/Repositories initialised below
	a := &Trains{
		Base: app.NewBase(ctx, authService, logger, cfg),
		db:   db,
	}

	a.Repositories = repositories.New(a.db)
	a.Services = services.New(client, a.Repositories.SavedCommutes)
	a.journeyWS = trainshttp.NewJourneyWS(
		a.Ctx, logger, []string{cfg.WebURL}, a.Services.JourneyDetail,
	)
	a.Services.WatchJourneys(a.journeyWS)

	return a
}

// Start begins pushing fresh detail to watched journeys.
func (a *Trains) Start() error {
	go a.journeyWS.Run(a.Ctx)
	return nil
}

func (a *Trains) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Trains) GetName() string { return "trains" }

func (a *Trains) GetDisplayName() string { return "Trains" }

func (a *Trains) GetDomain() string { return "" }
