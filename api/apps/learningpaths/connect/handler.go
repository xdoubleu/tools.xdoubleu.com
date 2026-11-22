// Package connect serves the learningpaths ConnectRPC API.
package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// Handler serves the LearningPathsService RPCs.
type Handler struct {
	paths *services.LearningPathService
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *services.LearningPathService) *Handler {
	return &Handler{paths: svc}
}

var _ learningpathsv1connect.LearningPathsServiceHandler = (*Handler)(nil)

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	return user.ID, nil
}

func parsePathID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.Errorf(
			connect.CodeInvalidArgument, "invalid learning path ID",
		)
	}
	return id, nil
}
