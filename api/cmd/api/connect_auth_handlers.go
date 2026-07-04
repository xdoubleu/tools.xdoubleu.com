package main

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/internal/models"
)

func isRelativeURL(url string) bool {
	return len(url) > 0 && url[0] == '/' && (len(url) == 1 || url[1] != '/')
}

func (h *authConnectHandler) SignIn(
	ctx context.Context,
	req *connect.Request[authv1.SignInRequest],
) (*connect.Response[authv1.SignInResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("email is required"),
		)
	}
	if req.Msg.Password == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("password is required"),
		)
	}
	if req.Msg.Redirect != "" && !isRelativeURL(req.Msg.Redirect) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("redirect must be a relative URL"),
		)
	}

	accessToken, refreshToken, err := h.app.auth.SignInWithEmail(
		ctx,
		req.Msg.Email,
		req.Msg.Password,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	factorID, hasMFA := h.app.auth.HasVerifiedTOTP(ctx, *accessToken)
	if !hasMFA {
		resp := connect.NewResponse(&authv1.SignInResponse{})
		if err = h.completeMFA(
			resp.Header(), *accessToken, *refreshToken, req.Msg.RememberMe,
		); err != nil {
			return nil, err
		}
		return resp, nil
	}

	resp := connect.NewResponse(&authv1.SignInResponse{NeedsMfa: true})
	h.setMFACookies(
		resp.Header(),
		*accessToken,
		*refreshToken,
		req.Msg.RememberMe,
		req.Msg.Redirect,
	)
	//nolint:gosec // Secure is conditionally set based on environment
	resp.Header().Add("Set-Cookie", (&http.Cookie{
		Name:     mfaFactorIDCookieName,
		Value:    factorID.String(),
		MaxAge:   int(mfaCookieTTL.Seconds()),
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   h.secure(),
		Path:     "/",
	}).String())
	return resp, nil
}

func (h *authConnectHandler) ForgotPassword(
	ctx context.Context,
	req *connect.Request[authv1.ForgotPasswordRequest],
) (*connect.Response[authv1.ForgotPasswordResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("email is required"),
		)
	}
	_ = h.app.auth.ForgotPassword(
		ctx,
		req.Msg.Email,
		h.app.config.WebURL+"/auth/reset-password",
	)
	return connect.NewResponse(&authv1.ForgotPasswordResponse{}), nil
}

func (h *authConnectHandler) ExchangeToken(
	ctx context.Context,
	req *connect.Request[authv1.ExchangeTokenRequest],
) (*connect.Response[authv1.ExchangeTokenResponse], error) {
	if req.Msg.AccessToken == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("access_token is required"),
		)
	}
	if req.Msg.RefreshToken == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("refresh_token is required"),
		)
	}

	if _, err := h.app.auth.GetUser(ctx, req.Msg.AccessToken); err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("invalid or expired token"),
		)
	}

	resp := connect.NewResponse(&authv1.ExchangeTokenResponse{})
	if err := h.completeMFA(
		resp.Header(), req.Msg.AccessToken, req.Msg.RefreshToken, true,
	); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *authConnectHandler) UpdatePassword(
	ctx context.Context,
	req *connect.Request[authv1.UpdatePasswordRequest],
) (*connect.Response[authv1.UpdatePasswordResponse], error) {
	if req.Msg.NewPassword == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("new_password is required"),
		)
	}

	accessToken, err := h.parseCookie(req.Header(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("not signed in"),
		)
	}

	if err = h.app.auth.UpdatePassword(
		ctx, accessToken.Value, req.Msg.NewPassword,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&authv1.UpdatePasswordResponse{}), nil
}

func (h *authConnectHandler) SignOut(
	ctx context.Context,
	req *connect.Request[authv1.SignOutRequest],
) (*connect.Response[authv1.SignOutResponse], error) {
	accessToken, err := h.parseCookie(req.Header(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("not signed in"),
		)
	}

	deleteAccess, deleteRefresh, err := h.app.auth.SignOut(
		ctx, accessToken.Value, h.secure(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := connect.NewResponse(&authv1.SignOutResponse{})
	resp.Header().Add("Set-Cookie", deleteAccess.String())
	resp.Header().Add("Set-Cookie", deleteRefresh.String())
	return resp, nil
}

func (h *authConnectHandler) GetCurrentUser(
	ctx context.Context,
	req *connect.Request[authv1.GetCurrentUserRequest],
) (*connect.Response[authv1.GetCurrentUserResponse], error) {
	resp := connect.NewResponse(&authv1.GetCurrentUserResponse{})

	var user *models.User
	if cookie, err := h.parseCookie(req.Header(), "accessToken"); err == nil {
		user, _ = h.app.auth.GetUser(ctx, cookie.Value)
	}

	if user == nil {
		user = h.tryRefreshToken(ctx, req.Header(), resp.Header())
		if user == nil {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				errors.New("not signed in"),
			)
		}
	}

	enrichedUser, dbErr := h.app.appUsersRepo.GetByID(ctx, user.ID)
	role := user.Role
	appAccess := []string{}
	if dbErr == nil {
		role = enrichedUser.Role
		appAccess = enrichedUser.AppAccess
	}

	resp.Msg.Role = string(role)
	resp.Msg.AppAccess = appAccess
	resp.Msg.HasMfa = user.HasMFA
	return resp, nil
}

// tryRefreshToken rotates the session via the shared RefreshSession path and
// adds the new cookies to the response; nil means the session is gone.
func (h *authConnectHandler) tryRefreshToken(
	ctx context.Context,
	reqHeader, respHeader http.Header,
) *models.User {
	refreshCookie, err := h.parseCookie(reqHeader, "refreshToken")
	if err != nil {
		return nil
	}

	user, accessCookie, refreshTokenCookie, err := h.app.auth.RefreshSession(
		ctx,
		refreshCookie.Value,
	)
	if err != nil {
		return nil
	}

	respHeader.Add("Set-Cookie", accessCookie.String())
	respHeader.Add("Set-Cookie", refreshTokenCookie.String())

	return user
}
