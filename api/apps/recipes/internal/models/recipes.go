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
}

// RecipeBookShare is a user the owner shares their whole recipe book with.
type RecipeBookShare struct {
	UserID      string
	CanEdit     bool
	DisplayName string
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
