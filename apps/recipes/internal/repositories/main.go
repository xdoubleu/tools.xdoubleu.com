package repositories

import "github.com/xdoubleu/essentia/v3/pkg/database/postgres"

type Repositories struct {
	Recipes  *RecipesRepository
	Plans    *PlansRepository
	Shopping *ShoppingRepository
}

func New(db postgres.DB) *Repositories {
	return &Repositories{
		Recipes:  &RecipesRepository{db: db},
		Plans:    &PlansRepository{db: db},
		Shopping: &ShoppingRepository{db: db},
	}
}
