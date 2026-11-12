package account

import (
	"context"
	"errors"
	"regexp"
	"slices"

	"connectrpc.com/connect/v2"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/auth"
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
		resp.TabPins, resp.TabPinsCustomised, _ = h.app.AppUsers.GetTabPins(
			ctx, user.ID,
		)
		resp.SuggestedPins, _ = h.app.AppUsers.MostOpenedApps(ctx, user.ID)
	}

	resp.Role = string(role)
	resp.AppAccess = appAccess
	resp.HasMfa = user.HasMFA
	resp.DisplayName = displayName
	resp.UserId = user.ID
	return resp, nil
}

// maxTabPins is how many apps fit the phone tab bar beside Today and Apps.
const maxTabPins = 3

// SetTabPins saves which apps the user pinned to the phone tab bar. A pin the
// user can't open is rejected; one they later lose access to is dropped by the
// web on read.
func (h *authConnectHandler) SetTabPins(
	ctx context.Context,
	req *authv1.SetTabPinsRequest,
) (*authv1.SetTabPinsResponse, error) {
	user, err := h.signedInUser(ctx)
	if err != nil {
		return nil, err
	}

	if len(req.TabPins) > maxTabPins {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "at most 3 apps can be pinned",
		)
	}
	enriched, err := h.accessOf(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, pin := range req.TabPins {
		if pin == "" || seen[pin] {
			return nil, connect.NewError(
				connect.CodeInvalidArgument, "pins must be distinct app ids",
			)
		}
		seen[pin] = true
		if err = requireApp(enriched, pin); err != nil {
			return nil, err
		}
	}

	pins := req.TabPins
	if pins == nil {
		pins = []string{}
	}
	if err = h.app.AppUsers.SetTabPins(ctx, user.ID, pins); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return &authv1.SetTabPinsResponse{}, nil
}

// signedInUser resolves the caller from the access token cookie.
func (h *authConnectHandler) signedInUser(ctx context.Context) (*models.User, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	accessToken, err := h.parseCookie(info.RequestHeader(), "accessToken")
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "not signed in")
	}
	user, err := h.app.Auth.GetUser(ctx, accessToken.Value)
	if err != nil || user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "not signed in")
	}
	return user, nil
}

// accessOf loads the user's role and grants.
func (h *authConnectHandler) accessOf(
	ctx context.Context,
	userID string,
) (*models.User, error) {
	enriched, err := h.app.AppUsers.GetByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return enriched, nil
}

// requireApp refuses an app the user can't open; admins can open any.
func requireApp(user *models.User, app string) error {
	if user.Role != models.RoleAdmin && !slices.Contains(user.AppAccess, app) {
		return connect.NewError(connect.CodePermissionDenied, "no access to "+app)
	}
	return nil
}

var appIDPattern = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// RecordAppOpen counts the user switching to an app. The counts only rank the
// default tab bar pins of someone who never chose.
func (h *authConnectHandler) RecordAppOpen(
	ctx context.Context,
	req *authv1.RecordAppOpenRequest,
) (*authv1.RecordAppOpenResponse, error) {
	user, err := h.signedInUser(ctx)
	if err != nil {
		return nil, err
	}
	if !appIDPattern.MatchString(req.App) {
		return nil, connect.NewError(connect.CodeInvalidArgument, "invalid app id")
	}
	enriched, err := h.accessOf(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if err = requireApp(enriched, req.App); err != nil {
		return nil, err
	}
	if err = h.app.AppUsers.RecordAppOpen(ctx, user.ID, req.App); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return &authv1.RecordAppOpenResponse{}, nil
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
