package account

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"connectrpc.com/connect/v2"

	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/models"
)

// dashboardTokenBytes is the random bytes behind a share token (256 bits).
const dashboardTokenBytes = 32

type dashboardConnectHandler struct {
	app *Deps
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
	req *dashboardv1.GetDashboardShareRequest,
) (*dashboardv1.GetDashboardShareResponse, error) {
	resp := &dashboardv1.GetDashboardShareResponse{}

	share, err := h.app.Shares.Get(
		ctx, h.userID(ctx), dashboardKindFromProto(req.Kind),
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		// Having no share link yet is a normal state, not an error.
		return resp, nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	resp.Share = protoDashboardShare(*share)
	return resp, nil
}

func (h *dashboardConnectHandler) CreateDashboardShare(
	ctx context.Context,
	req *dashboardv1.CreateDashboardShareRequest,
) (*dashboardv1.CreateDashboardShareResponse, error) {
	user, err := h.app.AppUsers.GetByID(ctx, h.userID(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	if user.DisplayName == "" {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition,
			"set a display name before sharing your dashboard",
		)
	}

	raw := make([]byte, dashboardTokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	share, err := h.app.Shares.Upsert(
		ctx, h.userID(ctx), dashboardKindFromProto(req.Kind), token,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &dashboardv1.CreateDashboardShareResponse{
		Share: protoDashboardShare(*share),
	}, nil
}

func (h *dashboardConnectHandler) DeleteDashboardShare(
	ctx context.Context,
	req *dashboardv1.DeleteDashboardShareRequest,
) (*dashboardv1.DeleteDashboardShareResponse, error) {
	err := h.app.Shares.Delete(
		ctx, h.userID(ctx), dashboardKindFromProto(req.Kind),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return &dashboardv1.DeleteDashboardShareResponse{}, nil
}

func (h *dashboardConnectHandler) SetDisplayName(
	ctx context.Context,
	req *dashboardv1.SetDisplayNameRequest,
) (*dashboardv1.SetDisplayNameResponse, error) {
	if req.DisplayName == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"display_name is required",
		)
	}

	err := h.app.AppUsers.SetDisplayName(ctx, h.userID(ctx), req.DisplayName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return &dashboardv1.SetDisplayNameResponse{}, nil
}
