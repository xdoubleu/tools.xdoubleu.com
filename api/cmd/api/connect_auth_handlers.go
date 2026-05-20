package main

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
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

	accessToken, _, err := h.app.services.Auth.SignInWithEmail(
		req.Msg.Email,
		req.Msg.Password,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	resp := connect.NewResponse(&authv1.SignInResponse{NeedsMfa: true})
	h.setMFACookies(resp.Header(), *accessToken, req.Msg.RememberMe, req.Msg.Redirect)

	factorID, hasMFA := h.app.services.Auth.HasVerifiedTOTP(*accessToken)
	if hasMFA {
		resp.Header().Add("Set-Cookie", (&http.Cookie{
			Name:     "mfaFactorID",
			Value:    factorID.String(),
			MaxAge:   int(mfaCookieTTL.Seconds()),
			SameSite: http.SameSiteStrictMode,
			HttpOnly: true,
			Secure:   h.secure(),
			Path:     "/",
		}).String())
		resp.Msg.EnrollMfa = false
	} else {
		resp.Msg.EnrollMfa = true
	}

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
	mfaFactorID, err := h.parseCookie(req.Header(), "mfaFactorID")
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
	_ context.Context,
	req *connect.Request[authv1.GetCurrentUserRequest],
) (*connect.Response[authv1.GetCurrentUserResponse], error) {
	cookie, err := h.parseCookie(req.Header(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("not signed in"),
		)
	}

	if _, err = h.app.services.Auth.GetUser(cookie.Value); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	return connect.NewResponse(&authv1.GetCurrentUserResponse{}), nil
}
