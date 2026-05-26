package repositories

import "github.com/xdoubleu/essentia/v4/pkg/database/postgres"

type Repositories struct {
	Recipes *RecipesRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Recipes: &RecipesRepository{db: db},
	}
}
