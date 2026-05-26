package shoppinglist

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
	"tools.xdoubleu.com/apps/shoppinglist/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

type ShoppingList struct {
	app.Base
	services *services.Services
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
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
	a.services = services.New(repositories.New(db), authService)

	return a
}

func (a *ShoppingList) ApplyMigrations(_ context.Context, _ *pgxpool.Pool) error {
	return nil // no own schema; queries mealplans and recipes schemas
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
