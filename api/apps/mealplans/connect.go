package mealplans

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/gen/mealplans/v1/mealplansv1connect"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

const hoursPerDay = 24

type mealplansConnectHandler struct {
	app *MealPlans
}

var _ mealplansv1connect.MealPlansServiceHandler = (*mealplansConnectHandler)(nil)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

func mapError(err error) error {
	return connecttools.MapError(err)
}

func protoPlanMeal(m *models.PlanMeal) *mealplansv1.PlanMeal {
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
	}
	if m.RecipeID != nil && m.RecipeName != "" {
		pb.Recipe = &recipesv1.Recipe{
			Id:   recipeID,
			Name: m.RecipeName,
		}
	}
	return pb
}

func protoPlan(p *models.Plan) *mealplansv1.Plan {
	if p == nil {
		return nil
	}
	meals := make([]*mealplansv1.PlanMeal, len(p.Meals))
	for i := range p.Meals {
		meals[i] = protoPlanMeal(&p.Meals[i])
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

func protoPlans(list []models.Plan) []*mealplansv1.Plan {
	result := make([]*mealplansv1.Plan, len(list))
	for i := range list {
		result[i] = protoPlan(&list[i])
	}
	return result
}

func protoSharedUsers(list []models.PlanSharedUser) []*mealplansv1.PlanSharedUser {
	result := make([]*mealplansv1.PlanSharedUser, len(list))
	for i, u := range list {
		result[i] = &mealplansv1.PlanSharedUser{
			UserId:      u.UserID,
			CanEdit:     u.CanEdit,
			DisplayName: u.DisplayName,
		}
	}
	return result
}
