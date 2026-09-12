package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth          auth.Service
	LearningPaths *LearningPathService
}

func New(
	_ *slog.Logger,
	repos *repositories.Repositories,
	authService auth.Service,
) *Services {
	return &Services{
		Auth:          authService,
		LearningPaths: &LearningPathService{repo: repos.LearningPaths},
	}
}
