package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	SlotBreakfast = "breakfast"
	SlotNoon      = "noon"
	SlotEvening   = "evening"
)

type Plan struct {
	ID            uuid.UUID
	OwnerUserID   string
	FamilyID      uuid.UUID
	Name          string
	ICalToken     uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CanEdit       bool
	ICalHideSlots []string
	ICalHidePast  bool
	Meals         []PlanMeal
}

type RecipeSuggestion struct {
	RecipeID uuid.UUID
	Servings int
}

type PlanMeal struct {
	ID                      uuid.UUID
	PlanID                  uuid.UUID
	MealDate                time.Time
	MealSlot                string
	RecipeID                *uuid.UUID
	CustomName              string
	Servings                int
	RecipeName              string
	ExcludeFromShoppingList bool
}
