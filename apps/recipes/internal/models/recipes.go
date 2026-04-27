package models

import (
	"time"

	"github.com/google/uuid"
)

type Recipe struct {
	ID           uuid.UUID
	UserID       string
	Name         string
	Description  string
	BaseServings int
	IsShared     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Ingredients  []Ingredient
}

type Ingredient struct {
	ID        uuid.UUID
	RecipeID  uuid.UUID
	Name      string
	Amount    float64
	Unit      string
	SortOrder int
}

type Plan struct {
	ID          uuid.UUID
	OwnerUserID string
	Name        string
	StartDate   time.Time
	EndDate     time.Time
	ICalToken   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CanEdit     bool
	Meals       []PlanMeal
}

type PlanMeal struct {
	ID       uuid.UUID
	PlanID   uuid.UUID
	MealDate time.Time
	MealSlot string
	RecipeID uuid.UUID
	Servings int
	Recipe   *Recipe
}

type ShoppingItem struct {
	Name   string
	Amount float64
	Unit   string
}
