package main

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/internal/models"
)

func isRelativeURL(url string) bool {
	return len(url) > 0 && url[0] == '/' && (len(url) == 1 || url[1] != '/')
}

func (h *authConnectHandler) SignIn(
	_ context.Context,
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

	accessToken, refreshToken, err := h.app.services.Auth.SignInWithEmail(
		req.Msg.Email,
		req.Msg.Password,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	factorID, hasMFA := h.app.services.Auth.HasVerifiedTOTP(*accessToken)
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

func (h *authConnectHandler) MFAEnroll(
	_ context.Context,
	req *connect.Request[authv1.MFAEnrollRequest],
) (*connect.Response[authv1.MFAEnrollResponse], error) {
	mfaToken, err := h.parseCookie(req.Header(), "mfaToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("mfa token required"),
		)
	}

	enrollment, err := h.app.services.Auth.EnrollTOTP(mfaToken.Value)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&authv1.MFAEnrollResponse{
		QrSvg:    enrollment.TOTP.QRCode,
		Secret:   enrollment.TOTP.Secret,
		FactorId: enrollment.ID.String(),
	}), nil
}

func (h *authConnectHandler) MFAEnrollVerify(
	_ context.Context,
	req *connect.Request[authv1.MFAEnrollVerifyRequest],
) (*connect.Response[authv1.MFAEnrollVerifyResponse], error) {
	mfaToken, err := h.parseCookie(req.Header(), "mfaToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("mfa token required"),
		)
	}

	factorID, parseErr := uuid.Parse(req.Msg.FactorId)
	if parseErr != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid factor id"),
		)
	}

	challenge, err := h.app.services.Auth.ChallengeMFA(mfaToken.Value, factorID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accessToken, refreshToken, err := h.app.services.Auth.VerifyMFA(
		mfaToken.Value, factorID, challenge.ID, req.Msg.Code,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	rememberMe := false
	if c, cErr := h.parseCookie(req.Header(), "mfaRememberMe"); cErr == nil {
		rememberMe = c.Value == "1"
	}

	resp := connect.NewResponse(&authv1.MFAEnrollVerifyResponse{})
	if err = h.completeMFA(
		resp.Header(), *accessToken, *refreshToken, rememberMe,
	); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *authConnectHandler) MFAEnrollSkip(
	_ context.Context,
	req *connect.Request[authv1.MFAEnrollSkipRequest],
) (*connect.Response[authv1.MFAEnrollSkipResponse], error) {
	mfaToken, err := h.parseCookie(req.Header(), "mfaToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("mfa token required"),
		)
	}
	mfaRefreshToken, err := h.parseCookie(req.Header(), "mfaRefreshToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("mfa refresh token required"),
		)
	}

	rememberMe := false
	if c, cErr := h.parseCookie(req.Header(), "mfaRememberMe"); cErr == nil {
		rememberMe = c.Value == "1"
	}

	resp := connect.NewResponse(&authv1.MFAEnrollSkipResponse{})
	if err = h.completeMFA(
		resp.Header(), mfaToken.Value, mfaRefreshToken.Value, rememberMe,
	); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *authConnectHandler) MFAChallenge(
	_ context.Context,
	req *connect.Request[authv1.MFAChallengeRequest],
) (*connect.Response[authv1.MFAChallengeResponse], error) {
	mfaToken, err := h.parseCookie(req.Header(), "mfaToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("mfa token required"),
		)
	}
	mfaFactorID, err := h.parseCookie(req.Header(), mfaFactorIDCookieName)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("mfa factor id required"),
		)
	}

	factorID, parseErr := uuid.Parse(mfaFactorID.Value)
	if parseErr != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid factor id"),
		)
	}

	challenge, err := h.app.services.Auth.ChallengeMFA(mfaToken.Value, factorID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accessToken, refreshToken, err := h.app.services.Auth.VerifyMFA(
		mfaToken.Value, factorID, challenge.ID, req.Msg.Code,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	rememberMe := false
	if c, cErr := h.parseCookie(req.Header(), "mfaRememberMe"); cErr == nil {
		rememberMe = c.Value == "1"
	}

	resp := connect.NewResponse(&authv1.MFAChallengeResponse{})
	if err = h.completeMFA(
		resp.Header(), *accessToken, *refreshToken, rememberMe,
	); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *authConnectHandler) ForgotPassword(
	_ context.Context,
	req *connect.Request[authv1.ForgotPasswordRequest],
) (*connect.Response[authv1.ForgotPasswordResponse], error) {
	if req.Msg.Email == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("email is required"),
		)
	}
	_ = h.app.services.Auth.ForgotPassword(req.Msg.Email)
	return connect.NewResponse(&authv1.ForgotPasswordResponse{}), nil
}

func (h *authConnectHandler) SignOut(
	_ context.Context,
	req *connect.Request[authv1.SignOutRequest],
) (*connect.Response[authv1.SignOutResponse], error) {
	accessToken, err := h.parseCookie(req.Header(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("not signed in"),
		)
	}

	deleteAccess, deleteRefresh, err := h.app.services.Auth.SignOut(
		accessToken.Value, h.secure(),
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
		user, _ = h.app.services.Auth.GetUser(cookie.Value)
	}

	if user == nil {
		newAccess, err := h.tryRefreshToken(req.Header(), resp.Header())
		if err != nil {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				errors.New("not signed in"),
			)
		}
		user, err = h.app.services.Auth.GetUser(newAccess)
		if err != nil || user == nil {
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
	return resp, nil
}

func (h *authConnectHandler) tryRefreshToken(
	reqHeader, respHeader http.Header,
) (string, error) {
	refreshCookie, err := h.parseCookie(reqHeader, "refreshToken")
	if err != nil {
		return "", errors.New("no refresh token")
	}

	newAccess, newRefresh, err := h.app.services.Auth.SignInWithRefreshToken(
		refreshCookie.Value,
	)
	if err != nil {
		return "", err
	}

	secure := h.secure()
	accessCookie, err := h.app.services.Auth.CreateCookie(
		models.AccessScope, *newAccess, h.app.config.AccessExpiry, secure,
	)
	if err != nil {
		return "", err
	}
	respHeader.Add("Set-Cookie", accessCookie.String())

	var refreshTokenCookie *http.Cookie
	refreshTokenCookie, err = h.app.services.Auth.CreateCookie(
		models.RefreshScope, *newRefresh, h.app.config.RefreshExpiry, secure,
	)
	if err != nil {
		return "", err
	}
	respHeader.Add("Set-Cookie", refreshTokenCookie.String())

	return *newAccess, nil
}
