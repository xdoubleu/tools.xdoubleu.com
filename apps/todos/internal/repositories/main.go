package repositories

import "github.com/xdoubleu/essentia/v4/pkg/database/postgres"

type Repositories struct {
	Tasks      *TasksRepository
	Settings   *SettingsRepository
	Sections   *SectionsRepository
	Policies   *PoliciesRepository
	Workspaces *WorkspacesRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Tasks:      &TasksRepository{db: db},
		Settings:   &SettingsRepository{db: db},
		Sections:   &SectionsRepository{db: db},
		Policies:   &PoliciesRepository{db: db},
		Workspaces: &WorkspacesRepository{db: db},
	}
}
