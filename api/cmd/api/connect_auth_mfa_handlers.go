package main

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
)

func (h *authConnectHandler) MFAEnroll(
	ctx context.Context,
	_ *authv1.MFAEnrollRequest,
) (*authv1.MFAEnrollResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	// Accept mfaToken (sign-in flow) or accessToken (settings flow).
	tokenCookie, err := h.parseCookie(info.RequestHeader(), mfaTokenCookieName)
	if err != nil {
		tokenCookie, err = h.parseCookie(info.RequestHeader(), "accessToken")
		if err != nil {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				"authentication required",
			)
		}
	}

	enrollment, err := h.app.auth.EnrollTOTP(ctx, tokenCookie.Value)
	if err != nil {
		return nil, connect.NewError(authErrorCode(err), err.Error()).WithCause(err)
	}

	return &authv1.MFAEnrollResponse{
		QrSvg:    enrollment.QRSVG,
		Secret:   enrollment.Secret,
		FactorId: enrollment.ID.String(),
	}, nil
}

func (h *authConnectHandler) MFAEnrollVerify(
	ctx context.Context,
	req *authv1.MFAEnrollVerifyRequest,
) (*authv1.MFAEnrollVerifyResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	// Accept mfaToken (sign-in flow) or accessToken (settings flow).
	tokenCookie, err := h.parseCookie(info.RequestHeader(), mfaTokenCookieName)
	isSettingsFlow := err != nil
	if isSettingsFlow {
		tokenCookie, err = h.parseCookie(info.RequestHeader(), "accessToken")
		if err != nil {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				"authentication required",
			)
		}
	}

	factorID, parseErr := uuid.Parse(req.FactorId)
	if parseErr != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid factor id",
		)
	}

	challenge, err := h.app.auth.ChallengeMFA(ctx, tokenCookie.Value, factorID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	accessToken, refreshToken, err := h.app.auth.VerifyMFA(
		ctx, tokenCookie.Value, factorID, challenge.ID, req.Code,
	)
	if err != nil {
		return nil, connect.NewError(signInErrorCode(err), err.Error()).WithCause(err)
	}

	rememberMe := false
	if isSettingsFlow {
		// Preserve the user's existing persistent session if they had one.
		if _, cookieErr := h.parseCookie(
			info.RequestHeader(),
			"refreshToken",
		); cookieErr == nil {
			rememberMe = true
		}
	} else {
		if c, cErr := h.parseCookie(
			info.RequestHeader(),
			mfaRememberMeCookieName,
		); cErr == nil {
			rememberMe = c.Value == "1"
		}
	}

	// First enrollment: return the one-time recovery codes now.
	recoveryCodes, codesErr := h.app.auth.GenerateRecoveryCodes(ctx, *accessToken)
	if codesErr != nil {
		return nil, connect.NewError(connect.CodeInternal, codesErr.Error()).
			WithCause(codesErr)
	}

	resp := &authv1.MFAEnrollVerifyResponse{
		RecoveryCodes: recoveryCodes,
	}
	if err = h.completeMFA(
		info.ResponseHeader(),
		*accessToken,
		*refreshToken,
		rememberMe,
	); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *authConnectHandler) RegenerateRecoveryCodes(
	ctx context.Context,
	req *authv1.RegenerateRecoveryCodesRequest,
) (*authv1.RegenerateRecoveryCodesResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	accessToken, err := h.parseCookie(info.RequestHeader(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"not signed in",
		)
	}

	if err = h.app.auth.VerifyCurrentFactor(
		ctx, accessToken.Value, req.Code,
	); err != nil {
		return nil, connect.NewError(authErrorCode(err), err.Error()).WithCause(err)
	}

	codes, err := h.app.auth.GenerateRecoveryCodes(ctx, accessToken.Value)
	if err != nil {
		return nil, connect.NewError(authErrorCode(err), err.Error()).WithCause(err)
	}

	return &authv1.RegenerateRecoveryCodesResponse{
		RecoveryCodes: codes,
	}, nil
}

func (h *authConnectHandler) MFAEnrollSkip(
	ctx context.Context,
	_ *authv1.MFAEnrollSkipRequest,
) (*authv1.MFAEnrollSkipResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	mfaToken, err := h.parseCookie(info.RequestHeader(), "mfaToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"mfa token required",
		)
	}
	// Skipping is only for users with no factor; others must pass the challenge.
	if _, hasMFA := h.app.auth.HasVerifiedTOTP(ctx, mfaToken.Value); hasMFA {
		return nil, connect.NewError(
			connect.CodePermissionDenied,
			"mfa challenge required",
		)
	}
	mfaRefreshToken, err := h.parseCookie(info.RequestHeader(), "mfaRefreshToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"mfa refresh token required",
		)
	}

	rememberMe := false
	if c, cErr := h.parseCookie(
		info.RequestHeader(),
		mfaRememberMeCookieName,
	); cErr == nil {
		rememberMe = c.Value == "1"
	}

	resp := &authv1.MFAEnrollSkipResponse{}
	if err = h.completeMFA(
		info.ResponseHeader(),
		mfaToken.Value,
		mfaRefreshToken.Value,
		rememberMe,
	); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *authConnectHandler) MFAChallenge(
	ctx context.Context,
	req *authv1.MFAChallengeRequest,
) (*authv1.MFAChallengeResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	mfaToken, err := h.parseCookie(info.RequestHeader(), "mfaToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"mfa token required",
		)
	}
	mfaFactorID, err := h.parseCookie(info.RequestHeader(), mfaFactorIDCookieName)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"mfa factor id required",
		)
	}

	factorID, parseErr := uuid.Parse(mfaFactorID.Value)
	if parseErr != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid factor id",
		)
	}

	challenge, err := h.app.auth.ChallengeMFA(ctx, mfaToken.Value, factorID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	accessToken, refreshToken, err := h.app.auth.VerifyMFA(
		ctx, mfaToken.Value, factorID, challenge.ID, req.Code,
	)
	if err != nil {
		return nil, connect.NewError(signInErrorCode(err), err.Error()).WithCause(err)
	}

	rememberMe := false
	if c, cErr := h.parseCookie(
		info.RequestHeader(),
		mfaRememberMeCookieName,
	); cErr == nil {
		rememberMe = c.Value == "1"
	}

	resp := &authv1.MFAChallengeResponse{}
	if err = h.completeMFA(
		info.ResponseHeader(),
		*accessToken,
		*refreshToken,
		rememberMe,
	); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *authConnectHandler) MFAUnenroll(
	ctx context.Context,
	req *authv1.MFAUnenrollRequest,
) (*authv1.MFAUnenrollResponse, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	accessToken, err := h.parseCookie(info.RequestHeader(), "accessToken")
	if err != nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"not signed in",
		)
	}

	factorID, hasMFA := h.app.auth.HasVerifiedTOTP(ctx, accessToken.Value)
	if !hasMFA {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition,
			"MFA is not enabled",
		)
	}

	if err = h.app.auth.VerifyCurrentFactor(
		ctx, accessToken.Value, req.Code,
	); err != nil {
		return nil, connect.NewError(authErrorCode(err), err.Error()).WithCause(err)
	}

	if err = h.app.auth.UnenrollTOTP(ctx, accessToken.Value, factorID); err != nil {
		return nil, connect.NewError(authErrorCode(err), err.Error()).WithCause(err)
	}

	return &authv1.MFAUnenrollResponse{}, nil
}
