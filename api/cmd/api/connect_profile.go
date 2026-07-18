package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	profilev1 "tools.xdoubleu.com/gen/profile/v1"
	"tools.xdoubleu.com/gen/profile/v1/profilev1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

// profileTokenBytes is the number of random bytes behind a profile share
// token (256 bits, URL-safe base64 in links).
const profileTokenBytes = 32

type profileConnectHandler struct {
	app *Application
}

var _ profilev1connect.ProfileServiceHandler = (*profileConnectHandler)(nil)

func (h *profileConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	return u.ID
}

func protoProfileShare(s models.ProfileShare) *profilev1.ProfileShare {
	return &profilev1.ProfileShare{
		Token:     s.Token,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
	}
}

func profileAppFromProto(app profilev1.ProfileApp) models.ProfileApp {
	if app == profilev1.ProfileApp_PROFILE_APP_GAMES {
		return models.ProfileAppGames
	}
	return models.ProfileAppReading
}

func (h *profileConnectHandler) GetProfileShare(
	ctx context.Context,
	req *connect.Request[profilev1.GetProfileShareRequest],
) (*connect.Response[profilev1.GetProfileShareResponse], error) {
	resp := &profilev1.GetProfileShareResponse{}

	share, err := h.app.profileSharesRepo.Get(
		ctx, h.userID(ctx), profileAppFromProto(req.Msg.App),
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		// Having no share link yet is a normal state, not an error.
		return connect.NewResponse(resp), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp.Share = protoProfileShare(*share)
	return connect.NewResponse(resp), nil
}

func (h *profileConnectHandler) CreateProfileShare(
	ctx context.Context,
	req *connect.Request[profilev1.CreateProfileShareRequest],
) (*connect.Response[profilev1.CreateProfileShareResponse], error) {
	user, err := h.app.appUsersRepo.GetByID(ctx, h.userID(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if user.DisplayName == "" {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("set a display name before sharing your profile"),
		)
	}

	raw := make([]byte, profileTokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	share, err := h.app.profileSharesRepo.Upsert(
		ctx, h.userID(ctx), profileAppFromProto(req.Msg.App), token,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&profilev1.CreateProfileShareResponse{
		Share: protoProfileShare(*share),
	}), nil
}

func (h *profileConnectHandler) DeleteProfileShare(
	ctx context.Context,
	req *connect.Request[profilev1.DeleteProfileShareRequest],
) (*connect.Response[profilev1.DeleteProfileShareResponse], error) {
	err := h.app.profileSharesRepo.Delete(
		ctx, h.userID(ctx), profileAppFromProto(req.Msg.App),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&profilev1.DeleteProfileShareResponse{}), nil
}

func (h *profileConnectHandler) SetDisplayName(
	ctx context.Context,
	req *connect.Request[profilev1.SetDisplayNameRequest],
) (*connect.Response[profilev1.SetDisplayNameResponse], error) {
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
	return connect.NewResponse(&profilev1.SetDisplayNameResponse{}), nil
}
