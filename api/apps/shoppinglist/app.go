package shoppinglist

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
	"tools.xdoubleu.com/apps/shoppinglist/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	sharedrepositories "tools.xdoubleu.com/internal/repositories"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type ShoppingList struct {
	app.Base
	services *services.Services
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	familyRepo *sharedrepositories.FamilyRepository,
) *ShoppingList {
	//nolint:exhaustruct //services initialised below
	a := &ShoppingList{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
		),
	}
	a.services = services.New(repositories.New(db), authService, familyRepo)

	return a
}

func (a *ShoppingList) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *ShoppingList) Start() error {
	return nil
}

func (a *ShoppingList) GetName() string {
	return "shoppinglist"
}

func (a *ShoppingList) GetDisplayName() string {
	return "Shopping List"
}
