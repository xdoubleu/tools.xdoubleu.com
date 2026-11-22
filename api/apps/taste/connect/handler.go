// Package connect serves the taste ConnectRPC API.
package connect

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/taste/internal/services"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// JobRunner runs a queued job ahead of its schedule.
type JobRunner interface {
	ForceRun(id string)
}

// Handler serves the TasteService RPCs.
type Handler struct {
	svc  *services.Services
	jobs JobRunner
}

// NewHandler returns a Handler backed by svc; jobs runs the generator on a
// refresh.
func NewHandler(svc *services.Services, jobs JobRunner) *Handler {
	return &Handler{svc: svc, jobs: jobs}
}

var _ tastev1connect.TasteServiceHandler = (*Handler)(nil)

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	return user.ID, nil
}

func mapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.Code(services.ErrNotConfigured, connect.CodeFailedPrecondition),
		connecttools.Code(services.ErrNoProfile, connect.CodeFailedPrecondition),
		connecttools.Code(services.ErrUpstream, connect.CodeUnavailable),
		connecttools.Code(services.ErrPickNotFound, connect.CodeNotFound),
	)
}

// scopeOf resolves the scope of the caller's request.
func (h *Handler) scopeOf(ctx context.Context, shared bool) (services.Scope, error) {
	uid, err := userID(ctx)
	if err != nil {
		return services.Scope{}, err
	}
	sc, err := h.svc.Scope.Resolve(ctx, uid, shared)
	if err != nil {
		return services.Scope{}, mapError(err)
	}
	return sc, nil
}
