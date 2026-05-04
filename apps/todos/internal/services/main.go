package services

import (
	"log/slog"

	"tools.xdoubleu.com/apps/todos/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth       auth.Service
	Tasks      *TaskService
	Settings   *SettingsService
	Sections   *SectionsService
	Policies   *PoliciesService
	Workspaces *WorkspacesService
}

func New(
	_ *slog.Logger,
	repos *repositories.Repositories,
	authService auth.Service,
) *Services {
	return &Services{
		Auth: authService,
		Tasks: &TaskService{
			tasks:    repos.Tasks,
			settings: repos.Settings,
			sections: repos.Sections,
		},
		Settings:   &SettingsService{settings: repos.Settings},
		Sections:   &SectionsService{sections: repos.Sections},
		Policies:   &PoliciesService{policies: repos.Policies},
		Workspaces: &WorkspacesService{workspaces: repos.Workspaces},
	}
}
