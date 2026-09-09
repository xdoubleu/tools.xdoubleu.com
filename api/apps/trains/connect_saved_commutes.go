package trains

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
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
	_ *connect.Request[trainsv1.ListSavedCommutesRequest],
) (*connect.Response[trainsv1.ListSavedCommutesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, fmt.Errorf("user not authenticated"),
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
	return connect.NewResponse(&trainsv1.ListSavedCommutesResponse{
		SavedCommutes: out,
	}), nil
}

func (h *trainsConnectHandler) CreateSavedCommute(
	ctx context.Context,
	req *connect.Request[trainsv1.CreateSavedCommuteRequest],
) (*connect.Response[trainsv1.CreateSavedCommuteResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, fmt.Errorf("user not authenticated"),
		)
	}

	created, err := h.app.Services.SavedCommutes.Create(
		ctx, user.ID,
		req.Msg.GetLabel(),
		req.Msg.GetOriginStopId(),
		req.Msg.GetDestinationStopId(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&trainsv1.CreateSavedCommuteResponse{
		SavedCommute: protoSavedCommute(&created),
	}), nil
}

func (h *trainsConnectHandler) UpdateSavedCommute(
	ctx context.Context,
	req *connect.Request[trainsv1.UpdateSavedCommuteRequest],
) (*connect.Response[trainsv1.UpdateSavedCommuteResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, fmt.Errorf("invalid saved commute id"),
		)
	}

	updated, err := h.app.Services.SavedCommutes.Update(
		ctx, user.ID, id, req.Msg.GetLabel(), int(req.Msg.GetPosition()),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&trainsv1.UpdateSavedCommuteResponse{
		SavedCommute: protoSavedCommute(&updated),
	}), nil
}

func (h *trainsConnectHandler) DeleteSavedCommute(
	ctx context.Context,
	req *connect.Request[trainsv1.DeleteSavedCommuteRequest],
) (*connect.Response[trainsv1.DeleteSavedCommuteResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, fmt.Errorf("invalid saved commute id"),
		)
	}

	if err = h.app.Services.SavedCommutes.Delete(ctx, user.ID, id); err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&trainsv1.DeleteSavedCommuteResponse{}), nil
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
		StopId: stopID,
		NameNl: stop.NameNL,
		NameFr: stop.NameFR,
		NameEn: stop.NameEN,
	}
}
