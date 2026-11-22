// Package connect serves the mealplans ConnectRPC API.
package connect

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/mealplans/internal/services"
	"tools.xdoubleu.com/gen/mealplans/v1/mealplansv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// Handler serves the MealPlansService RPCs.
type Handler struct {
	plans *services.PlanService
}

// NewHandler returns a Handler backed by plans.
func NewHandler(plans *services.PlanService) *Handler {
	return &Handler{plans: plans}
}

var _ mealplansv1connect.MealPlansServiceHandler = (*Handler)(nil)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

func userID(ctx context.Context) (string, error) {
	user := getUser(ctx)
	if user == nil {
		return "", connect.Errorf(connect.CodeUnauthenticated, "user not authenticated")
	}
	return user.ID, nil
}
