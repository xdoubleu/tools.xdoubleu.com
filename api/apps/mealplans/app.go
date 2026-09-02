package mealplans

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/mealplans/internal/repositories"
	"tools.xdoubleu.com/apps/mealplans/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	sharedrepositories "tools.xdoubleu.com/internal/repositories"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type MealPlans struct {
	app.Base
	services *services.Services
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	familyRepo *sharedrepositories.FamilyRepository,
) *MealPlans {
	//nolint:exhaustruct //services initialised below
	a := &MealPlans{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
		),
	}
	a.services = services.New(a.Logger, repositories.New(db), authService, familyRepo)

	return a
}

func (a *MealPlans) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *MealPlans) Start() error {
	return nil
}

func (a *MealPlans) GetName() string {
	return "mealplans"
}

func (a *MealPlans) GetDisplayName() string {
	return "Meal Plans"
}
