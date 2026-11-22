// Package connect serves the trains ConnectRPC API.
package connect

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
	"tools.xdoubleu.com/gen/trains/v1/trainsv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// Handler serves the TrainService RPCs.
type Handler struct {
	svc *services.Services
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *services.Services) *Handler {
	return &Handler{svc: svc}
}

var _ trainsv1connect.TrainServiceHandler = (*Handler)(nil)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

func mapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.Code(services.ErrUnknownStation, connect.CodeNotFound),
		connecttools.Code(irail.ErrRateLimited, connect.CodeUnavailable),
		connecttools.Code(services.ErrInvalidCommute, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidJourneyID, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidDate, connect.CodeInvalidArgument),
	)
}
