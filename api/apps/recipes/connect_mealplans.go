package recipes

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
)

type mealplansConnectHandler struct {
	app *Recipes
}

var _ recipesv1connect.MealPlansServiceHandler = (*mealplansConnectHandler)(nil)

// ── Proto conversion helpers for meal plans ────────────────────────────────

func protoPlan(p *models.Plan) *recipesv1.Plan {
	if p == nil {
		return nil
	}
	meals := make([]*recipesv1.PlanMeal, len(p.Meals))
	for i := range p.Meals {
		meals[i] = protoPlanMeal(&p.Meals[i])
	}
	return &recipesv1.Plan{
		Id:          p.ID.String(),
		OwnerUserId: p.OwnerUserID,
		Name:        p.Name,
		IcalToken:   p.ICalToken.String(),
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
		CanEdit:     p.CanEdit,
		Meals:       meals,
	}
}

func protoPlans(list []models.Plan) []*recipesv1.Plan {
	result := make([]*recipesv1.Plan, len(list))
	for i := range list {
		result[i] = protoPlan(&list[i])
	}
	return result
}

func protoPlanMeal(m *models.PlanMeal) *recipesv1.PlanMeal {
	if m == nil {
		return nil
	}
	recipeID := ""
	if m.RecipeID != nil {
		recipeID = m.RecipeID.String()
	}
	return &recipesv1.PlanMeal{
		Id:         m.ID.String(),
		PlanId:     m.PlanID.String(),
		MealDate:   m.MealDate.Format(time.DateOnly),
		MealSlot:   m.MealSlot,
		RecipeId:   recipeID,
		CustomName: m.CustomName,
		Servings:   int32(m.Servings), //nolint:gosec // int32 safe for domain values
		Recipe:     protoRecipe(m.Recipe),
	}
}

func protoShoppingItem(item *models.ShoppingItem) *recipesv1.ShoppingItem {
	if item == nil {
		return nil
	}
	return &recipesv1.ShoppingItem{
		Name:   item.Name,
		Amount: item.Amount,
		Unit:   item.Unit,
	}
}

func protoShoppingItems(items []models.ShoppingItem) []*recipesv1.ShoppingItem {
	result := make([]*recipesv1.ShoppingItem, len(items))
	for i := range items {
		result[i] = protoShoppingItem(&items[i])
	}
	return result
}

// ── Plan RPCs ──────────────────────────────────────────────────────────────

func (h *mealplansConnectHandler) ListPlans(
	ctx context.Context,
	_ *connect.Request[recipesv1.ListPlansRequest],
) (*connect.Response[recipesv1.ListPlansResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	list, err := h.app.services.Plans.List(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.ListPlansResponse{
		Plans: protoPlans(list),
	}), nil
}

func (h *mealplansConnectHandler) GetPlan(
	ctx context.Context,
	req *connect.Request[recipesv1.GetPlanRequest],
) (*connect.Response[recipesv1.GetPlanResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	plan, err := h.app.services.Plans.Get(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	offset := int(req.Msg.Offset)
	if offset == 0 {
		offset = 1
	}

	prevOffset := offset - 1
	if prevOffset < 1 {
		prevOffset = 0
	}

	windowStart := time.Now().UTC()
	windowEnd := windowStart.AddDate(0, 0, daysPerWeek-1)

	recipeList := make([]*recipesv1.Recipe, 0)

	return connect.NewResponse(&recipesv1.GetPlanResponse{
		Plan:        protoPlan(plan),
		Recipes:     recipeList,
		IcalUrl:     "",
		IsOwner:     plan.OwnerUserID == user.ID,
		Offset:      int32(offset),
		PrevOffset:  int32(prevOffset),
		NextOffset:  int32(offset + 1),
		WindowStart: windowStart.Format(time.RFC3339),
		WindowEnd:   windowEnd.Format(time.RFC3339),
	}), nil
}

func (h *mealplansConnectHandler) CreatePlan(
	ctx context.Context,
	req *connect.Request[recipesv1.CreatePlanRequest],
) (*connect.Response[recipesv1.CreatePlanResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	plan := models.Plan{ //nolint:exhaustruct // other fields optional
		OwnerUserID: user.ID,
		Name:        req.Msg.Name,
	}

	created, err := h.app.services.Plans.Create(ctx, user.ID, plan)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.CreatePlanResponse{
		Plan: protoPlan(created),
	}), nil
}

func (h *mealplansConnectHandler) UpdatePlan(
	ctx context.Context,
	req *connect.Request[recipesv1.UpdatePlanRequest],
) (*connect.Response[recipesv1.UpdatePlanResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	plan := models.Plan{ //nolint:exhaustruct // other fields optional
		ID:            id,
		Name:          req.Msg.Name,
		ICalHideSlots: req.Msg.IcalHideSlots,
		ICalHidePast:  req.Msg.IcalHidePast,
	}

	err = h.app.services.Plans.Update(ctx, user.ID, plan)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.UpdatePlanResponse{}), nil
}

func (h *mealplansConnectHandler) DeletePlan(
	ctx context.Context,
	req *connect.Request[recipesv1.DeletePlanRequest],
) (*connect.Response[recipesv1.DeletePlanResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	err = h.app.services.Plans.Delete(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.DeletePlanResponse{}), nil
}

func (h *mealplansConnectHandler) AddMeal(
	ctx context.Context,
	req *connect.Request[recipesv1.AddMealRequest],
) (*connect.Response[recipesv1.AddMealResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	mealDate, err := time.Parse(time.DateOnly, req.Msg.MealDate)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal date"),
		)
	}

	var recipeID *uuid.UUID
	if req.Msg.RecipeId != "" {
		id, parseErr := uuid.Parse(req.Msg.RecipeId)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf("invalid recipe ID"),
			)
		}
		recipeID = &id
	}

	servings := 2
	if req.Msg.Servings > 0 {
		servings = int(req.Msg.Servings)
	}

	customName := req.Msg.CustomName

	meal := models.PlanMeal{ //nolint:exhaustruct // other fields optional
		PlanID:     planID,
		MealDate:   mealDate,
		MealSlot:   req.Msg.MealSlot,
		RecipeID:   recipeID,
		CustomName: customName,
		Servings:   servings,
	}

	err = h.app.services.Plans.AddMeal(ctx, planID, user.ID, meal)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.AddMealResponse{}), nil
}

func (h *mealplansConnectHandler) DeleteMeal(
	ctx context.Context,
	req *connect.Request[recipesv1.DeleteMealRequest],
) (*connect.Response[recipesv1.DeleteMealResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	mealID, err := uuid.Parse(req.Msg.MealId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal ID"),
		)
	}

	err = h.app.services.Plans.DeleteMeal(ctx, mealID, planID, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.DeleteMealResponse{}), nil
}

func (h *mealplansConnectHandler) SharePlan(
	ctx context.Context,
	req *connect.Request[recipesv1.SharePlanRequest],
) (*connect.Response[recipesv1.SharePlanResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	err = h.app.services.Plans.Share(
		ctx,
		planID,
		user.ID,
		req.Msg.ContactUserId,
		req.Msg.CanEdit,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.SharePlanResponse{}), nil
}

//nolint:dupl // structurally identical to UnshareRecipe; different types
func (h *mealplansConnectHandler) UnsharePlan(
	ctx context.Context,
	req *connect.Request[recipesv1.UnsharePlanRequest],
) (*connect.Response[recipesv1.UnsharePlanResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	if req.Msg.TargetUserId == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("target user ID is required"),
		)
	}

	err = h.app.services.Plans.Unshare(ctx, planID, user.ID, req.Msg.TargetUserId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.UnsharePlanResponse{}), nil
}

// ── Shopping List RPC ──────────────────────────────────────────────────────

func (h *mealplansConnectHandler) GetShoppingList(
	ctx context.Context,
	req *connect.Request[recipesv1.GetShoppingListRequest],
) (*connect.Response[recipesv1.GetShoppingListResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	plan, err := h.app.services.Plans.Get(ctx, planID, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	today := time.Now().UTC().Truncate(hoursPerDay * time.Hour)
	end := today.AddDate(0, 0, daysPerWeek-1)

	items, err := h.app.services.Shopping.GetList(ctx, planID, today, end)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.GetShoppingListResponse{
		Plan:  protoPlan(plan),
		Items: protoShoppingItems(items),
	}), nil
}
