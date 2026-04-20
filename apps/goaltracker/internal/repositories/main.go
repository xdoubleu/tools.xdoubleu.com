package repositories

import (
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
)

type Repositories struct {
	Goals        *GoalRepository
	States       *StateRepository
	Progress     *ProgressRepository
	Goodreads    *GoodreadsRepository
	Steam        *SteamRepository
	Integrations *IntegrationsRepository
}

func New(db postgres.DB) *Repositories {
	goals := &GoalRepository{db: db}
	states := &StateRepository{db: db}
	progress := &ProgressRepository{db: db}
	goodreads := &GoodreadsRepository{db: db}
	steam := &SteamRepository{db: db}
	integrations := &IntegrationsRepository{db: db}

	return &Repositories{
		Goals:        goals,
		States:       states,
		Progress:     progress,
		Goodreads:    goodreads,
		Steam:        steam,
		Integrations: integrations,
	}
}
