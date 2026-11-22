// Package protomap maps mealplans domain models to their proto messages.
package protomap

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
)

// PlanMeal maps a meal; nil stays nil.
func PlanMeal(m *models.PlanMeal) *mealplansv1.PlanMeal {
	if m == nil {
		return nil
	}
	recipeID := ""
	if m.RecipeID != nil {
		recipeID = m.RecipeID.String()
	}
	servings := int32(m.Servings) //nolint:gosec // int32 safe for domain values
	pb := &mealplansv1.PlanMeal{
		Id:                      m.ID.String(),
		PlanId:                  m.PlanID.String(),
		MealDate:                m.MealDate.Format(time.DateOnly),
		MealSlot:                m.MealSlot,
		RecipeId:                recipeID,
		CustomName:              m.CustomName,
		Servings:                servings,
		ExcludeFromShoppingList: m.ExcludeFromShoppingList,
		CreatedAtMicros:         m.CreatedAt.UnixMicro(),
	}
	if m.RecipeID != nil && m.RecipeName != "" {
		pb.RecipeRef = &mealplansv1.RecipeRef{
			Id:   recipeID,
			Name: m.RecipeName,
		}
	}
	return pb
}

// Plan maps a plan with its meals; nil stays nil.
func Plan(p *models.Plan) *mealplansv1.Plan {
	if p == nil {
		return nil
	}
	meals := make([]*mealplansv1.PlanMeal, len(p.Meals))
	for i := range p.Meals {
		meals[i] = PlanMeal(&p.Meals[i])
	}
	return &mealplansv1.Plan{
		Id:            p.ID.String(),
		OwnerUserId:   p.OwnerUserID,
		Name:          p.Name,
		IcalToken:     p.ICalToken.String(),
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     p.UpdatedAt.Format(time.RFC3339),
		CanEdit:       p.CanEdit,
		Meals:         meals,
		IcalHideSlots: p.ICalHideSlots,
		IcalHidePast:  p.ICalHidePast,
	}
}

// Plans maps a list of plans.
func Plans(list []models.Plan) []*mealplansv1.Plan {
	result := make([]*mealplansv1.Plan, len(list))
	for i := range list {
		result[i] = Plan(&list[i])
	}
	return result
}

// ListPlans builds the ListPlans response shared by the RPC and MCP tool.
func ListPlans(list []models.Plan, hasMore bool) *mealplansv1.ListPlansResponse {
	return &mealplansv1.ListPlansResponse{Plans: Plans(list), HasMore: hasMore}
}

// ICalURL is the public calendar feed path for a plan's token.
func ICalURL(token uuid.UUID) string {
	return fmt.Sprintf("/mealplans/ical/%s.ics", token)
}

// GetPlan builds the GetPlan response shared by the RPC and MCP tool.
func GetPlan(
	plan *models.Plan,
	userID string,
	offset int,
	windowStart, windowEnd time.Time,
) *mealplansv1.GetPlanResponse {
	return &mealplansv1.GetPlanResponse{
		Plan:        Plan(plan),
		IcalUrl:     ICalURL(plan.ICalToken),
		IsOwner:     plan.OwnerUserID == userID,
		Offset:      int32(offset),     //nolint:gosec // pagination offset fits int32
		PrevOffset:  int32(offset - 1), //nolint:gosec // pagination offset fits int32
		NextOffset:  int32(offset + 1), //nolint:gosec // pagination offset fits int32
		WindowStart: windowStart.Format(time.RFC3339),
		WindowEnd:   windowEnd.Format(time.RFC3339),
	}
}

// SuggestRecipes builds the SuggestRecipes response shared by the RPC and MCP
// tool.
func SuggestRecipes(
	suggestions []models.RecipeSuggestion,
) *mealplansv1.SuggestRecipesResponse {
	out := make([]*mealplansv1.RecipeSuggestion, len(suggestions))
	for i, s := range suggestions {
		out[i] = &mealplansv1.RecipeSuggestion{
			RecipeId: s.RecipeID.String(),
			Servings: int32(s.Servings), //nolint:gosec // int32 safe for domain values
			Fresh:    s.Fresh,
		}
	}
	return &mealplansv1.SuggestRecipesResponse{Suggestions: out}
}
