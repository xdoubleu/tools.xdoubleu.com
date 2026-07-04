package main

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
)

func (h *authConnectHandler) MFAEnroll(
	ctx context.Context,
	req *connect.Request[authv1.MFAEnrollRequest],
) (*connect.Response[authv1.MFAEnrollResponse], error) {
	// Accept mfaToken (sign-in flow) or accessToken (settings flow).
	tokenCookie, err := h.parseCookie(req.Header(), mfaTokenCookieName)
	if err != nil {
		tokenCookie, err = h.parseCookie(req.Header(), "accessToken")
		if err != nil {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				errors.New("authentication required"),
			)
		}
	}

	enrollment, err := h.app.auth.EnrollTOTP(ctx, tokenCookie.Value)
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
	ctx context.Context,
	req *connect.Request[authv1.MFAEnrollVerifyRequest],
) (*connect.Response[authv1.MFAEnrollVerifyResponse], error) {
	// Accept mfaToken (sign-in flow) or accessToken (settings flow).
	tokenCookie, err := h.parseCookie(req.Header(), mfaTokenCookieName)
	isSettingsFlow := err != nil
	if isSettingsFlow {
		tokenCookie, err = h.parseCookie(req.Header(), "accessToken")
		if err != nil {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				errors.New("authentication required"),
			)
		}
	}

	factorID, parseErr := uuid.Parse(req.Msg.FactorId)
	if parseErr != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid factor id"),
		)
	}

	challenge, err := h.app.auth.ChallengeMFA(ctx, tokenCookie.Value, factorID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accessToken, refreshToken, err := h.app.auth.VerifyMFA(
		ctx, tokenCookie.Value, factorID, challenge.ID, req.Msg.Code,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	rememberMe := false
	if isSettingsFlow {
		// Preserve the user's existing persistent session if they had one.
		if _, cookieErr := h.parseCookie(req.Header(), "refreshToken"); cookieErr == nil {
			rememberMe = true
		}
	} else {
		if c, cErr := h.parseCookie(req.Header(), mfaRememberMeCookieName); cErr == nil {
			rememberMe = c.Value == "1"
		}
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
	if c, cErr := h.parseCookie(req.Header(), mfaRememberMeCookieName); cErr == nil {
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
	ctx context.Context,
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

	challenge, err := h.app.auth.ChallengeMFA(ctx, mfaToken.Value, factorID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accessToken, refreshToken, err := h.app.auth.VerifyMFA(
		ctx, mfaToken.Value, factorID, challenge.ID, req.Msg.Code,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	rememberMe := false
	if c, cErr := h.parseCookie(req.Header(), mfaRememberMeCookieName); cErr == nil {
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

func (h *authConnectHandler) MFAUnenroll(
	ctx context.Context,
	req *connect.Request[authv1.MFAUnenrollRequest],
) (*connect.Response[authv1.MFAUnenrollResponse], error) {
	accessToken, err := h.parseCookie(req.Header(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("not signed in"),
		)
	}

	factorID, hasMFA := h.app.auth.HasVerifiedTOTP(ctx, accessToken.Value)
	if !hasMFA {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition,
			errors.New("MFA is not enabled"),
		)
	}

	if err = h.app.auth.UnenrollTOTP(ctx, accessToken.Value, factorID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&authv1.MFAUnenrollResponse{}), nil
}
