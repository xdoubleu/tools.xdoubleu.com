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

type PlanSharedUser struct {
	UserID      string
	CanEdit     bool
	DisplayName string
}

type Plan struct {
	ID            uuid.UUID
	OwnerUserID   string
	Name          string
	ICalToken     uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CanEdit       bool
	ICalHideSlots []string
	ICalHidePast  bool
	Meals         []PlanMeal
	SharedWith    []PlanSharedUser
}

type PlanMeal struct {
	ID         uuid.UUID
	PlanID     uuid.UUID
	MealDate   time.Time
	MealSlot   string
	RecipeID   *uuid.UUID
	CustomName string
	Servings   int
	RecipeName string
}
