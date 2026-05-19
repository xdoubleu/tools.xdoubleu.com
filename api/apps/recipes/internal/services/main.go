package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/recipes/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth     auth.Service
	Recipes  *RecipeService
	Plans    *PlanService
	Shopping *ShoppingService
}

func New(
	_ *slog.Logger,
	repos *repositories.Repositories,
	authService auth.Service,
) *Services {
	return &Services{
		Auth:     authService,
		Recipes:  &RecipeService{repo: repos.Recipes},
		Plans:    &PlanService{repo: repos.Plans},
		Shopping: &ShoppingService{repo: repos.Shopping},
	}
}
