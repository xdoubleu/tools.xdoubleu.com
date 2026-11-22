package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	"tools.xdoubleu.com/apps/mealplans/internal/protomap"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *Handler) ListPlans(
	ctx context.Context,
	req *mealplansv1.ListPlansRequest,
) (*mealplansv1.ListPlansResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	list, hasMore, err := h.plans.List(
		ctx, uid, req.Limit, req.Offset,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return protomap.ListPlans(list, hasMore), nil
}

func (h *Handler) GetPlan(
	ctx context.Context,
	req *mealplansv1.GetPlanRequest,
) (*mealplansv1.GetPlanResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	offset := int(req.Offset)
	plan, windowStart, windowEnd, err := h.plans.GetWithWeek(
		ctx, id, uid, offset,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return protomap.GetPlan(plan, uid, offset, windowStart, windowEnd), nil
}

func (h *Handler) UpdatePlan(
	ctx context.Context,
	req *mealplansv1.UpdatePlanRequest,
) (*mealplansv1.UpdatePlanResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
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

	if err = h.plans.Update(ctx, uid, plan); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.UpdatePlanResponse{}, nil
}

func (h *Handler) RotateICalToken(
	ctx context.Context,
	req *mealplansv1.RotateICalTokenRequest,
) (*mealplansv1.RotateICalTokenResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	token, err := h.plans.RotateICalToken(ctx, id, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.RotateICalTokenResponse{
		IcalUrl: protomap.ICalURL(token),
	}, nil
}
