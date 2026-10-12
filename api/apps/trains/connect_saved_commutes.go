package trains

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/trains/internal/models"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

func (h *trainsConnectHandler) ListSavedCommutes(
	ctx context.Context,
	_ *trainsv1.ListSavedCommutesRequest,
) (*trainsv1.ListSavedCommutesResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	list, err := h.app.Services.SavedCommutes.List(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*trainsv1.SavedCommute, len(list))
	for i := range list {
		out[i] = protoSavedCommute(&list[i])
	}
	return &trainsv1.ListSavedCommutesResponse{
		SavedCommutes: out,
	}, nil
}

func (h *trainsConnectHandler) CreateSavedCommute(
	ctx context.Context,
	req *trainsv1.CreateSavedCommuteRequest,
) (*trainsv1.CreateSavedCommuteResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	created, err := h.app.Services.SavedCommutes.Create(
		ctx, user.ID,
		req.GetLabel(),
		req.GetOriginStopId(),
		req.GetDestinationStopId(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.CreateSavedCommuteResponse{
		SavedCommute: protoSavedCommute(&created),
	}, nil
}

func (h *trainsConnectHandler) UpdateSavedCommute(
	ctx context.Context,
	req *trainsv1.UpdateSavedCommuteRequest,
) (*trainsv1.UpdateSavedCommuteResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument, "invalid saved commute id",
		)
	}

	updated, err := h.app.Services.SavedCommutes.Update(
		ctx, user.ID, id, req.GetLabel(), int(req.GetPosition()),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.UpdateSavedCommuteResponse{
		SavedCommute: protoSavedCommute(&updated),
	}, nil
}

func (h *trainsConnectHandler) DeleteSavedCommute(
	ctx context.Context,
	req *trainsv1.DeleteSavedCommuteRequest,
) (*trainsv1.DeleteSavedCommuteResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument, "invalid saved commute id",
		)
	}

	if err = h.app.Services.SavedCommutes.Delete(ctx, user.ID, id); err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.DeleteSavedCommuteResponse{}, nil
}

func protoSavedCommute(sc *models.SavedCommute) *trainsv1.SavedCommute {
	return &trainsv1.SavedCommute{
		Id:          sc.ID.String(),
		Label:       sc.Label,
		Origin:      protoStation(sc.OriginStopID, &sc.Origin),
		Destination: protoStation(sc.DestinationStopID, &sc.Destination),
		Position:    int32(sc.Position), //nolint:gosec // small list ordering value
	}
}

func protoStation(stopID string, stop *models.Stop) *trainsv1.Station {
	return &trainsv1.Station{
		StopId:      stopID,
		NameNl:      stop.NameNL,
		NameFr:      stop.NameFR,
		NameEn:      stop.NameEN,
		DisplayName: stop.DisplayName,
	}
}
