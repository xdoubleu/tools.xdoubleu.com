// Package connect serves the dashboard app's ConnectRPC services.
package connect

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/dashboard/internal/protomap"
	"tools.xdoubleu.com/apps/dashboard/internal/services"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// TodayHandler serves dashboard.v1.TodayService.
type TodayHandler struct {
	today *services.TodayService
}

var _ dashboardv1connect.TodayServiceHandler = (*TodayHandler)(nil)

// NewTodayHandler returns a TodayHandler backed by the Today service.
func NewTodayHandler(today *services.TodayService) *TodayHandler {
	return &TodayHandler{today: today}
}

func (h *TodayHandler) GetToday(
	ctx context.Context,
	_ *dashboardv1.GetTodayRequest,
) (*dashboardv1.GetTodayResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	return protomap.Today(h.today.Get(ctx, user)), nil
}
