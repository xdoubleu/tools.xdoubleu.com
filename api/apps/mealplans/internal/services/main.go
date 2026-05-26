package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/mealplans/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth  auth.Service
	Plans *PlanService
}

func New(
	_ *slog.Logger,
	repos *repositories.Repositories,
	authService auth.Service,
) *Services {
	return &Services{
		Auth:  authService,
		Plans: &PlanService{repo: repos.Plans},
	}
}
