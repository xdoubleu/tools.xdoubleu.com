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
	sections := &SectionsService{sections: repos.Sections}
	policies := &PoliciesService{policies: repos.Policies}
	workspaces := &WorkspacesService{workspaces: repos.Workspaces}

	return &Services{
		Auth: authService,
		Tasks: &TaskService{
			tasks:    repos.Tasks,
			settings: repos.Settings,
			sections: repos.Sections,
		},
		Settings: &SettingsService{
			settings:   repos.Settings,
			sections:   sections,
			policies:   policies,
			workspaces: workspaces,
		},
		Sections:   sections,
		Policies:   policies,
		Workspaces: workspaces,
	}
}
