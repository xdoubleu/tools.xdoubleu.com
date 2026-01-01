package repositories

import (
	"github.com/XDoubleU/essentia/pkg/database/postgres"
)

type Repositories struct {
	Goals     *GoalRepository
	States    *StateRepository
	Progress  *ProgressRepository
	Goodreads *GoodreadsRepository
	Steam     *SteamRepository
}

func New(db postgres.DB) *Repositories {
	goals := &GoalRepository{db: db}
	states := &StateRepository{db: db}
	progress := &ProgressRepository{db: db}
	goodreads := &GoodreadsRepository{db: db}
	steam := &SteamRepository{db: db}

	return &Repositories{
		Goals:     goals,
		States:    states,
		Progress:  progress,
		Goodreads: goodreads,
		Steam:     steam,
	}
}
