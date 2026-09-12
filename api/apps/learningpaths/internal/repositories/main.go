package repositories

import "tools.xdoubleu.com/internal/database/postgres"

type Repositories struct {
	LearningPaths *LearningPathsRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		LearningPaths: &LearningPathsRepository{db: db},
	}
}
