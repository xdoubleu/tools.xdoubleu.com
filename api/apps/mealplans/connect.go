package mealplans

import (
	"context"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/gen/mealplans/v1/mealplansv1connect"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

const (
	daysPerWeek = 7
	hoursPerDay = 24
)

type mealplansConnectHandler struct {
	app *MealPlans
}

var _ mealplansv1connect.MealPlansServiceHandler = (*mealplansConnectHandler)(nil)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, database.ErrResourceNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	if errors.Is(err, database.ErrResourceConflict) {
		return connect.NewError(connect.CodeAlreadyExists, err)
	}
	var httpErr *iapp.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusBadRequest:
			return connect.NewError(connect.CodeInvalidArgument, err)
		case http.StatusNotFound:
			return connect.NewError(connect.CodeNotFound, err)
		case http.StatusConflict:
			return connect.NewError(connect.CodeAlreadyExists, err)
		default:
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewError(connect.CodeInternal, err)
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
