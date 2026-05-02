package models

import (
	"time"

	"github.com/google/uuid"
)

type Recipe struct {
	ID           uuid.UUID
	UserID       string
	Name         string
	Instructions string
	BaseServings int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Ingredients  []Ingredient
	SharedWith   []string
}

type Ingredient struct {
	ID        uuid.UUID
	RecipeID  uuid.UUID
	Name      string
	Amount    float64
	Unit      string
	SortOrder int
}

type PlanSharedUser struct {
	UserID      string
	CanEdit     bool
	DisplayName string
}

type Plan struct {
	ID          uuid.UUID
	OwnerUserID string
	Name        string
	ICalToken   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CanEdit     bool
	Meals       []PlanMeal
	SharedWith  []PlanSharedUser
}

type PlanMeal struct {
	ID         uuid.UUID
	PlanID     uuid.UUID
	MealDate   time.Time
	MealSlot   string
	RecipeID   *uuid.UUID
	CustomName string
	Servings   int
	Recipe     *Recipe
}

type ShoppingItem struct {
	Name   string
	Amount float64
	Unit   string
}
