package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/recipes/internal/repositories"
	"tools.xdoubleu.com/internal/platform/auth"
)

type Services struct {
	Auth    auth.Service
	Recipes *RecipeService
	Videos  *VideoService
}

func New(
	_ *slog.Logger,
	repos *repositories.Repositories,
	authService auth.Service,
	family familyStore,
	videos VideoFetcher,
) *Services {
	return &Services{
		Auth:    authService,
		Recipes: &RecipeService{repo: repos.Recipes, family: family},
		Videos:  NewVideoService(videos),
	}
}
