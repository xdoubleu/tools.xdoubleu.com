package mealplans

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *mealplansConnectHandler) ListPlans(
	ctx context.Context,
	req *mealplansv1.ListPlansRequest,
) (*mealplansv1.ListPlansResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	list, hasMore, err := h.app.services.Plans.List(
		ctx, user.ID, req.Limit, req.Offset,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	// Only an empty first page means no plans yet.
	if len(list) == 0 && req.Offset == 0 {
		created, createErr := h.app.services.Plans.Create(
			ctx,
			user.ID,
			models.Plan{ //nolint:exhaustruct // other fields optional
				OwnerUserID: user.ID,
				Name:        "My Meal Plan",
			},
		)
		if createErr != nil {
			return nil, connecttools.MapError(createErr)
		}
		list = []models.Plan{*created}
	}

	return &mealplansv1.ListPlansResponse{
		Plans:   protoPlans(list),
		HasMore: hasMore,
	}, nil
}

func (h *mealplansConnectHandler) GetPlan(
	ctx context.Context,
	req *mealplansv1.GetPlanRequest,
) (*mealplansv1.GetPlanResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	offset := int(req.Offset)
	plan, windowStart, windowEnd, err := h.app.services.Plans.GetWithWeek(
		ctx, id, user.ID, offset,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.GetPlanResponse{
		Plan:        protoPlan(plan),
		IcalUrl:     h.icalURL(plan.ICalToken),
		IsOwner:     plan.OwnerUserID == user.ID,
		Offset:      int32(offset),     //nolint:gosec // pagination offset fits int32
		PrevOffset:  int32(offset - 1), //nolint:gosec // pagination offset fits int32
		NextOffset:  int32(offset + 1), //nolint:gosec // pagination offset fits int32
		WindowStart: windowStart.Format(time.RFC3339),
		WindowEnd:   windowEnd.Format(time.RFC3339),
	}, nil
}

func (h *mealplansConnectHandler) UpdatePlan(
	ctx context.Context,
	req *mealplansv1.UpdatePlanRequest,
) (*mealplansv1.UpdatePlanResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	plan := models.Plan{ //nolint:exhaustruct // other fields optional
		ID:            id,
		Name:          req.Name,
		ICalHideSlots: req.IcalHideSlots,
		ICalHidePast:  req.IcalHidePast,
	}

	if err = h.app.services.Plans.Update(ctx, user.ID, plan); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.UpdatePlanResponse{}, nil
}

func (h *mealplansConnectHandler) RotateICalToken(
	ctx context.Context,
	req *mealplansv1.RotateICalTokenRequest,
) (*mealplansv1.RotateICalTokenResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	token, err := h.app.services.Plans.RotateICalToken(ctx, id, user.ID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.RotateICalTokenResponse{
		IcalUrl: h.icalURL(token),
	}, nil
}

func (h *mealplansConnectHandler) icalURL(token uuid.UUID) string {
	return fmt.Sprintf("/%s/ical/%s.ics", h.app.GetName(), token)
}
