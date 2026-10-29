package account

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/models"
)

func IsRelativeURL(url string) bool {
	return len(url) > 0 && url[0] == '/' &&
		(len(url) == 1 || (url[1] != '/' && url[1] != '\\'))
}

func (h *authConnectHandler) SignIn(
	ctx context.Context,
	req *authv1.SignInRequest,
) (*authv1.SignInResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	if req.Email == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"email is required",
		)
	}
	if req.Password == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"password is required",
		)
	}
	if req.Redirect != "" && !IsRelativeURL(req.Redirect) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"redirect must be a relative URL",
		)
	}

	accessToken, refreshToken, err := h.app.Auth.SignInWithEmail(
		ctx,
		req.Email,
		req.Password,
	)
	if err != nil {
		return nil, connect.NewError(SignInErrorCode(err), err.Error()).WithCause(err)
	}

	factorID, hasMFA := h.app.Auth.HasVerifiedTOTP(ctx, *accessToken)
	if !hasMFA {
		resp := &authv1.SignInResponse{}
		if err = h.completeMFA(
			info.ResponseHeader(),
			*accessToken,
			*refreshToken,
			req.RememberMe,
		); err != nil {
			return nil, err
		}
		return resp, nil
	}

	resp := &authv1.SignInResponse{NeedsMfa: true}
	h.setMFACookies(info.ResponseHeader(), *accessToken,
		*refreshToken,
		req.RememberMe,
		req.Redirect,
	)
	h.setMFAFactorIDCookie(info.ResponseHeader(), factorID)
	return resp, nil
}

func (h *authConnectHandler) ForgotPassword(
	ctx context.Context,
	req *authv1.ForgotPasswordRequest,
) (*authv1.ForgotPasswordResponse, error) {
	if req.Email == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"email is required",
		)
	}
	_ = h.app.Auth.ForgotPassword(
		ctx,
		req.Email,
		h.app.Config.WebURL+"/auth/reset-password",
	)
	return &authv1.ForgotPasswordResponse{}, nil
}

func (h *authConnectHandler) ResetPassword(
	ctx context.Context,
	req *authv1.ResetPasswordRequest,
) (*authv1.ResetPasswordResponse, error) {
	if req.Token == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"token is required",
		)
	}
	if req.NewPassword == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"new_password is required",
		)
	}

	if err := h.app.Auth.ResetPasswordWithToken(
		ctx, req.Token, req.NewPassword,
	); err != nil {
		code := connect.CodeUnauthenticated
		if errors.Is(err, auth.ErrPasswordTooShort) {
			code = connect.CodeInvalidArgument
		}
		return nil, connect.NewError(code, err.Error()).WithCause(err)
	}

	return &authv1.ResetPasswordResponse{}, nil
}

func (h *authConnectHandler) ExchangeToken(
	ctx context.Context,
	req *authv1.ExchangeTokenRequest,
) (*authv1.ExchangeTokenResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	if req.AccessToken == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"access_token is required",
		)
	}
	if req.RefreshToken == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"refresh_token is required",
		)
	}

	if err := h.app.Auth.ValidateAccessToken(req.AccessToken); err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"invalid or expired token",
		)
	}

	// Like SignIn: a verified TOTP factor must still be challenged, so a reset
	// can't bypass MFA.
	factorID, hasMFA := h.app.Auth.HasVerifiedTOTP(ctx, req.AccessToken)
	if hasMFA {
		resp := &authv1.ExchangeTokenResponse{NeedsMfa: true}
		h.setMFACookies(info.ResponseHeader(), req.AccessToken, req.RefreshToken, true, "")
		h.setMFAFactorIDCookie(info.ResponseHeader(), factorID)
		return resp, nil
	}

	resp := &authv1.ExchangeTokenResponse{}
	if err := h.completeMFA(
		info.ResponseHeader(),
		req.AccessToken,
		req.RefreshToken,
		true,
	); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *authConnectHandler) UpdatePassword(
	ctx context.Context,
	req *authv1.UpdatePasswordRequest,
) (*authv1.UpdatePasswordResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	if req.NewPassword == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"new_password is required",
		)
	}

	accessToken, err := h.parseCookie(info.RequestHeader(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"not signed in",
		)
	}

	if err = h.app.Auth.UpdatePassword(
		ctx, accessToken.Value, req.CurrentPassword, req.NewPassword,
	); err != nil {
		return nil, connect.NewError(AuthErrorCode(err), err.Error()).WithCause(err)
	}

	return &authv1.UpdatePasswordResponse{}, nil
}

func (h *authConnectHandler) SignOut(
	ctx context.Context,
	_ *authv1.SignOutRequest,
) (*authv1.SignOutResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	accessToken, err := h.parseCookie(info.RequestHeader(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"not signed in",
		)
	}

	deleteAccess, deleteRefresh, err := h.app.Auth.SignOut(
		ctx, accessToken.Value, h.secure(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	resp := &authv1.SignOutResponse{}
	info.ResponseHeader().Add("Set-Cookie", deleteAccess.String())
	info.ResponseHeader().Add("Set-Cookie", deleteRefresh.String())
	return resp, nil
}

func (h *authConnectHandler) GetCurrentUser(
	ctx context.Context,
	_ *authv1.GetCurrentUserRequest,
) (*authv1.GetCurrentUserResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	resp := &authv1.GetCurrentUserResponse{}

	var user *models.User
	if cookie, err := h.parseCookie(info.RequestHeader(), "accessToken"); err == nil {
		user, _ = h.app.Auth.GetUser(ctx, cookie.Value)
	}

	if user == nil {
		user = h.tryRefreshToken(ctx, info.RequestHeader(), info.ResponseHeader())
		if user == nil {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				"not signed in",
			)
		}
	}

	enrichedUser, dbErr := h.app.AppUsers.GetByID(ctx, user.ID)
	role := user.Role
	appAccess := []string{}
	displayName := ""
	if dbErr == nil {
		role = enrichedUser.Role
		appAccess = enrichedUser.AppAccess
		displayName = enrichedUser.DisplayName
	}

	resp.Role = string(role)
	resp.AppAccess = appAccess
	resp.HasMfa = user.HasMFA
	resp.DisplayName = displayName
	resp.UserId = user.ID
	return resp, nil
}

// tryRefreshToken rotates the session and sets new cookies; nil means the
// session is gone.
func (h *authConnectHandler) tryRefreshToken(
	ctx context.Context,
	reqHeader, respHeader *connect.Header,
) *models.User {
	refreshCookie, err := h.parseCookie(reqHeader, "refreshToken")
	if err != nil {
		return nil
	}

	user, accessCookie, refreshTokenCookie, err := h.app.Auth.RefreshSession(
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
