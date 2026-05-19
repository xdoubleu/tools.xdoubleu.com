package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth     auth.Service
	Calendar *CalendarService
}

func New(
	logger *slog.Logger,
	repos *repositories.Repositories,
	auth auth.Service,
) *Services {
	return &Services{
		Auth:     auth,
		Calendar: &CalendarService{logger: logger, repo: repos.Calendar},
	}
}
