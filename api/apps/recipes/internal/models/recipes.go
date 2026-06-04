package models

import (
	"time"

	"github.com/google/uuid"
)

type Recipe struct {
	ID            uuid.UUID
	UserID        string
	Name          string
	Instructions  string
	BaseServings  int
	BatchServings *int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Ingredients   []Ingredient
	SharedWith    []string
}

type Ingredient struct {
	ID        uuid.UUID
	RecipeID  uuid.UUID
	Name      string
	Amount    float64
	Unit      string
	SortOrder int
	GroupName *string
}
