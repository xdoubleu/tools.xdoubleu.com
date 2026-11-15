package models

import (
	"time"

	"github.com/google/uuid"
)

// Allowed category values; the empty string means uncategorised.
const (
	MealTypeLight = "light_meal"
	MealTypeMain  = "main_meal"
	EffortLow     = "low"
	EffortHigh    = "high"
)

// ValidMealType reports whether v is empty or a known meal type.
func ValidMealType(v string) bool {
	return v == "" || v == MealTypeLight || v == MealTypeMain
}

// ValidEffort reports whether v is empty or a known effort level.
func ValidEffort(v string) bool {
	return v == "" || v == EffortLow || v == EffortHigh
}

type Recipe struct {
	ID            uuid.UUID
	UserID        string
	FamilyID      uuid.UUID
	Name          string
	Instructions  string
	BaseServings  int
	BatchServings *int
	IsDraft       bool
	MealType      string
	Effort        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Ingredients   []Ingredient
}

type Ingredient struct {
	ID        uuid.UUID
	RecipeID  uuid.UUID
	Name      string
	Amount    float64
	Unit      string
	SortOrder int
	GroupName *string
	// AlternativeGroup links interchangeable options; the first is the default.
	AlternativeGroup *string
}
