package services

import (
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/recipes/internal/models"
)

const defaultServings = 2

// RecipeInput is a recipe as a caller supplies it, before normalisation.
type RecipeInput struct {
	ID            uuid.UUID
	Name          string
	Steps         []string
	BaseServings  int
	BatchServings *int
	IsDraft       bool
	MealType      string
	Effort        string
	Ingredients   []IngredientInput
}

// IngredientInput is one ingredient line. Its position is kept as the sort
// order, even when blank lines before it are dropped.
type IngredientInput struct {
	Name             string
	Amount           float64
	Unit             string
	GroupName        string
	AlternativeGroup string
}

// CreateOption adjusts a recipe before Create stores it.
type CreateOption func(*models.Recipe)

// AsAgentDraft stores the recipe as a draft, whatever the input says: agents
// propose recipes and the user publishes them (adr-0026).
func AsAgentDraft() CreateOption {
	return func(r *models.Recipe) { r.IsDraft = true }
}

// ScaledRecipe is a recipe with its ingredients scaled to Servings.
type ScaledRecipe struct {
	Recipe      *models.Recipe
	CanEdit     bool
	Servings    int
	Ingredients []models.Ingredient
}

// toRecipe normalises the input: steps are trimmed, blank ones dropped and
// joined by newlines, servings default to two, blank ingredients are dropped
// and blank group labels become nil.
func (in RecipeInput) toRecipe() models.Recipe {
	var steps []string
	for _, s := range in.Steps {
		if t := strings.TrimSpace(s); t != "" {
			steps = append(steps, t)
		}
	}

	servings := in.BaseServings
	if servings <= 0 {
		servings = defaultServings
	}

	var ingredients []models.Ingredient
	for i, ing := range in.Ingredients {
		if ing.Name == "" {
			continue
		}
		//nolint:exhaustruct // other fields set by storage
		ingredients = append(ingredients, models.Ingredient{
			Name:             ing.Name,
			Amount:           ing.Amount,
			Unit:             strings.TrimSpace(ing.Unit),
			SortOrder:        i,
			GroupName:        optionalTrimmed(ing.GroupName),
			AlternativeGroup: optionalTrimmed(ing.AlternativeGroup),
		})
	}

	//nolint:exhaustruct // owner, family and timestamps are set by the service
	return models.Recipe{
		ID:            in.ID,
		Name:          in.Name,
		Instructions:  strings.Join(steps, "\n"),
		BaseServings:  servings,
		BatchServings: in.BatchServings,
		IsDraft:       in.IsDraft,
		MealType:      in.MealType,
		Effort:        in.Effort,
		Ingredients:   ingredients,
	}
}

// optionalTrimmed returns v trimmed, or nil when blank.
func optionalTrimmed(v string) *string {
	t := strings.TrimSpace(v)
	if t == "" {
		return nil
	}
	return &t
}
