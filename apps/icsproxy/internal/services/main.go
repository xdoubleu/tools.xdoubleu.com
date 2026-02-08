package services

import (
	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth     auth.Service
	Calendar *CalendarService
}

func New(repos *repositories.Repositories, auth auth.Service) *Services {
	return &Services{
		Auth:     auth,
		Calendar: &CalendarService{repo: repos.Calendar},
	}
}
