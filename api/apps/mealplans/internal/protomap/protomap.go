// Package protomap maps mealplans domain models to their proto messages.
package protomap

import (
	"time"

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
