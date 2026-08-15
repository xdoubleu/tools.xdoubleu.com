package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"connectrpc.com/connect"

	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/models"
)

// dashboardTokenBytes is the number of random bytes behind a dashboard share
// token (256 bits, URL-safe base64 in links).
const dashboardTokenBytes = 32

type dashboardConnectHandler struct {
	app *Application
}

var _ dashboardv1connect.DashboardServiceHandler = (*dashboardConnectHandler)(nil)

func (h *dashboardConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	return u.ID
}

func protoDashboardShare(s models.ProfileShare) *dashboardv1.DashboardShare {
	return &dashboardv1.DashboardShare{
		Token:     s.Token,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
	}
}

func dashboardKindFromProto(kind dashboardv1.DashboardKind) models.DashboardKind {
	if kind == dashboardv1.DashboardKind_DASHBOARD_KIND_GAMES {
		return models.DashboardKindGames
	}
	return models.DashboardKindReading
}

func (h *dashboardConnectHandler) GetDashboardShare(
	ctx context.Context,
	req *connect.Request[dashboardv1.GetDashboardShareRequest],
) (*connect.Response[dashboardv1.GetDashboardShareResponse], error) {
	resp := &dashboardv1.GetDashboardShareResponse{}

	share, err := h.app.profileSharesRepo.Get(
		ctx, h.userID(ctx), dashboardKindFromProto(req.Msg.Kind),
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		// Having no share link yet is a normal state, not an error.
		return connect.NewResponse(resp), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp.Share = protoDashboardShare(*share)
	return connect.NewResponse(resp), nil
}

func (h *dashboardConnectHandler) CreateDashboardShare(
	ctx context.Context,
	req *connect.Request[dashboardv1.CreateDashboardShareRequest],
) (*connect.Response[dashboardv1.CreateDashboardShareResponse], error) {
	user, err := h.app.appUsersRepo.GetByID(ctx, h.userID(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if user.DisplayName == "" {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("set a display name before sharing your dashboard"),
		)
	}

	raw := make([]byte, dashboardTokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	share, err := h.app.profileSharesRepo.Upsert(
		ctx, h.userID(ctx), dashboardKindFromProto(req.Msg.Kind), token,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&dashboardv1.CreateDashboardShareResponse{
		Share: protoDashboardShare(*share),
	}), nil
}

func (h *dashboardConnectHandler) DeleteDashboardShare(
	ctx context.Context,
	req *connect.Request[dashboardv1.DeleteDashboardShareRequest],
) (*connect.Response[dashboardv1.DeleteDashboardShareResponse], error) {
	err := h.app.profileSharesRepo.Delete(
		ctx, h.userID(ctx), dashboardKindFromProto(req.Msg.Kind),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&dashboardv1.DeleteDashboardShareResponse{}), nil
}

func (h *dashboardConnectHandler) SetDisplayName(
	ctx context.Context,
	req *connect.Request[dashboardv1.SetDisplayNameRequest],
) (*connect.Response[dashboardv1.SetDisplayNameResponse], error) {
	if req.Msg.DisplayName == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("display_name is required"),
		)
	}

	err := h.app.appUsersRepo.SetDisplayName(ctx, h.userID(ctx), req.Msg.DisplayName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&dashboardv1.SetDisplayNameResponse{}), nil
}
