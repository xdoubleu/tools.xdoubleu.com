package repositories

import "tools.xdoubleu.com/internal/database/postgres"

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
