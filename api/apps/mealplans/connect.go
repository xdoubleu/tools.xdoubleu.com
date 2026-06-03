package mealplans

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
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

// ── Shared helpers ─────────────────────────────────────────────────────────

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

// ── Proto conversion helpers ───────────────────────────────────────────────

func protoPlanMeal(m *models.PlanMeal) *mealplansv1.PlanMeal {
	if m == nil {
		return nil
	}
	recipeID := ""
	if m.RecipeID != nil {
		recipeID = m.RecipeID.String()
	}
	pb := &mealplansv1.PlanMeal{
		Id:         m.ID.String(),
		PlanId:     m.PlanID.String(),
		MealDate:   m.MealDate.Format(time.DateOnly),
		MealSlot:   m.MealSlot,
		RecipeId:   recipeID,
		CustomName: m.CustomName,
		Servings:   int32(m.Servings), //nolint:gosec // int32 safe for domain values
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

// ── RPCs ───────────────────────────────────────────────────────────────────

func (h *mealplansConnectHandler) ListPlans(
	ctx context.Context,
	_ *connect.Request[mealplansv1.ListPlansRequest],
) (*connect.Response[mealplansv1.ListPlansResponse], error) {
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

	if len(list) == 0 {
		created, createErr := h.app.services.Plans.Create(
			ctx,
			user.ID,
			models.Plan{ //nolint:exhaustruct // other fields optional
				OwnerUserID: user.ID,
				Name:        "My Meal Plan",
			},
		)
		if createErr != nil {
			return nil, mapError(createErr)
		}
		list = []models.Plan{*created}
	}

	return connect.NewResponse(&mealplansv1.ListPlansResponse{
		Plans: protoPlans(list),
	}), nil
}

func (h *mealplansConnectHandler) GetPlan(
	ctx context.Context,
	req *connect.Request[mealplansv1.GetPlanRequest],
) (*connect.Response[mealplansv1.GetPlanResponse], error) {
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
	today := time.Now().UTC().Truncate(hoursPerDay * time.Hour)
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := today.AddDate(0, 0, 1-weekday)
	windowStart := monday.AddDate(0, 0, daysPerWeek*offset)
	windowEnd := windowStart.AddDate(0, 0, daysPerWeek-1)

	meals, err := h.app.services.Plans.GetMeals(
		ctx,
		id,
		user.ID,
		windowStart,
		windowEnd,
	)
	if err != nil {
		return nil, mapError(err)
	}
	plan.Meals = meals

	icalURL := fmt.Sprintf("/%s/ical/%s.ics", h.app.GetName(), plan.ICalToken)

	return connect.NewResponse(&mealplansv1.GetPlanResponse{
		Plan:        protoPlan(plan),
		Recipes:     []*recipesv1.Recipe{},
		IcalUrl:     icalURL,
		IsOwner:     plan.OwnerUserID == user.ID,
		Offset:      int32(offset),     //nolint:gosec // pagination offset fits int32
		PrevOffset:  int32(offset - 1), //nolint:gosec // pagination offset fits int32
		NextOffset:  int32(offset + 1), //nolint:gosec // pagination offset fits int32
		WindowStart: windowStart.Format(time.RFC3339),
		WindowEnd:   windowEnd.Format(time.RFC3339),
		SharedWith:  protoSharedUsers(plan.SharedWith),
	}), nil
}

func (h *mealplansConnectHandler) UpdatePlan(
	ctx context.Context,
	req *connect.Request[mealplansv1.UpdatePlanRequest],
) (*connect.Response[mealplansv1.UpdatePlanResponse], error) {
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

	if err = h.app.services.Plans.Update(ctx, user.ID, plan); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.UpdatePlanResponse{}), nil
}

func (h *mealplansConnectHandler) AddMeal(
	ctx context.Context,
	req *connect.Request[mealplansv1.AddMealRequest],
) (*connect.Response[mealplansv1.AddMealResponse], error) {
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

	if recipeID == nil && req.Msg.CustomName == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("recipe ID or custom name is required"),
		)
	}

	servings := 2
	if req.Msg.Servings > 0 {
		servings = int(req.Msg.Servings)
	}

	meal := models.PlanMeal{ //nolint:exhaustruct // other fields optional
		PlanID:     planID,
		MealDate:   mealDate,
		MealSlot:   req.Msg.MealSlot,
		RecipeID:   recipeID,
		CustomName: req.Msg.CustomName,
		Servings:   servings,
	}

	if err = h.app.services.Plans.AddMeal(ctx, planID, user.ID, meal); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.AddMealResponse{}), nil
}

func (h *mealplansConnectHandler) DeleteMeal(
	ctx context.Context,
	req *connect.Request[mealplansv1.DeleteMealRequest],
) (*connect.Response[mealplansv1.DeleteMealResponse], error) {
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

	if err = h.app.services.Plans.DeleteMeal(ctx, mealID, planID, user.ID); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.DeleteMealResponse{}), nil
}

func (h *mealplansConnectHandler) MoveMeal(
	ctx context.Context,
	req *connect.Request[mealplansv1.MoveMealRequest],
) (*connect.Response[mealplansv1.MoveMealResponse], error) {
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

	newDate, err := time.Parse(time.DateOnly, req.Msg.NewDate)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal date"),
		)
	}

	validSlots := map[string]bool{
		models.SlotBreakfast: true,
		models.SlotNoon:      true,
		models.SlotEvening:   true,
	}
	if !validSlots[req.Msg.NewSlot] {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal slot"),
		)
	}

	if err = h.app.services.Plans.MoveMeal(
		ctx, mealID, planID, user.ID, newDate, req.Msg.NewSlot,
	); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.MoveMealResponse{}), nil
}

func (h *mealplansConnectHandler) SharePlan(
	ctx context.Context,
	req *connect.Request[mealplansv1.SharePlanRequest],
) (*connect.Response[mealplansv1.SharePlanResponse], error) {
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

	if err = h.app.services.Plans.Share(
		ctx, planID, user.ID, req.Msg.ContactUserId, req.Msg.CanEdit,
	); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.SharePlanResponse{}), nil
}

func (h *mealplansConnectHandler) UnsharePlan(
	ctx context.Context,
	req *connect.Request[mealplansv1.UnsharePlanRequest],
) (*connect.Response[mealplansv1.UnsharePlanResponse], error) {
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

	if err = h.app.services.Plans.Unshare(
		ctx, planID, user.ID, req.Msg.TargetUserId,
	); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.UnsharePlanResponse{}), nil
}
