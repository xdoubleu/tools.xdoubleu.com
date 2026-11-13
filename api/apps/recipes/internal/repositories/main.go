package repositories

import "tools.xdoubleu.com/internal/kit/database/postgres"

type Repositories struct {
	Recipes *RecipesRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Recipes: &RecipesRepository{db: db},
	}
}
