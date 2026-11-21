// Package connect serves the movies ConnectRPC API.
package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/movies/internal/services"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// Handler serves the MoviesService RPCs.
type Handler struct {
	movies *services.MovieService
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *services.MovieService) *Handler {
	return &Handler{movies: svc}
}

var _ moviesv1connect.MoviesServiceHandler = (*Handler)(nil)

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	return user.ID, nil
}

// owner is the key the caller's list is stored under; see MovieService.Owner.
func (h *Handler) owner(ctx context.Context, shared bool) (string, error) {
	uid, err := userID(ctx)
	if err != nil || !shared {
		return uid, err
	}
	owner, err := h.movies.Owner(ctx, uid, shared)
	return owner, mapError(err)
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument, "invalid id",
		)
	}
	return id, nil
}

func mapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.Code(services.ErrNotConfigured, connect.CodeFailedPrecondition),
	)
}
