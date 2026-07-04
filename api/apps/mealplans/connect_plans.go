package mealplans

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
)

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
	windowStart := today.AddDate(0, 0, daysPerWeek*offset)
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
