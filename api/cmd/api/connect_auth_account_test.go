package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
)

func TestExchangeToken_Success(t *testing.T) {
	client := authClient(t)
	ctx, info := connect.NewClientContext(context.Background())
	resp, err := client.ExchangeToken(ctx, &authv1.ExchangeTokenRequest{
		AccessToken: accessToken.Value, RefreshToken: "refresh",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	setCookieHeaders := info.ResponseHeader().Values("Set-Cookie")
	assert.NotEmpty(t, setCookieHeaders)
}

func TestExchangeToken_NeedsMFA(t *testing.T) {
	// A verified TOTP factor must still be challenged before ExchangeToken grants
	// a session. mfaAccessToken's user has one.
	client := authClient(t)
	ctx, info := connect.NewClientContext(context.Background())
	resp, err := client.ExchangeToken(ctx, &authv1.ExchangeTokenRequest{
		AccessToken:  mfaAccessToken.Value,
		RefreshToken: "mfa-refresh",
	})
	require.NoError(t, err)
	assert.True(t, resp.NeedsMfa)

	setCookieHeaders := info.ResponseHeader().Values("Set-Cookie")
	sawMFACookie, sawSessionCookie := false, false
	for _, c := range setCookieHeaders {
		switch {
		case strings.HasPrefix(c, "mfaToken="):
			sawMFACookie = true
		case strings.HasPrefix(c, "accessToken="),
			strings.HasPrefix(c, "refreshToken="):
			sawSessionCookie = true
		}
	}
	assert.True(t, sawMFACookie, "expected mfaToken cookie to be set")
	assert.False(t, sawSessionCookie, "must not grant a full session before MFA")
}

func TestExchangeToken_InvalidToken(t *testing.T) {
	client := authClient(t)
	_, err := client.ExchangeToken(context.Background(), &authv1.ExchangeTokenRequest{
		AccessToken:  "bad-token",
		RefreshToken: "bad-refresh",
	})
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestExchangeToken_EmptyAccessToken(t *testing.T) {
	client := authClient(t)
	_, err := client.ExchangeToken(
		context.Background(),
		&authv1.ExchangeTokenRequest{AccessToken: "", RefreshToken: "refresh"},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestExchangeToken_EmptyRefreshToken(t *testing.T) {
	client := authClient(t)
	_, err := client.ExchangeToken(
		context.Background(),
		&authv1.ExchangeTokenRequest{AccessToken: "access", RefreshToken: ""},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestUpdatePassword_NoToken(t *testing.T) {
	client := authClient(t)
	_, err := client.UpdatePassword(
		context.Background(),
		&authv1.UpdatePasswordRequest{NewPassword: "newpass"},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestUpdatePassword_EmptyPassword(t *testing.T) {
	client := authClient(t)
	req := &authv1.UpdatePasswordRequest{NewPassword: ""}
	_, err := client.UpdatePassword(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestUpdatePassword_Success(t *testing.T) {
	// A throwaway user, so changing its password doesn't break shared fixtures.
	token := freshTestUser(t)
	client := authClient(t)
	req := &authv1.UpdatePasswordRequest{
		NewPassword: "newpassword123", CurrentPassword: testUserPassword,
	}
	_, err := client.UpdatePassword(
		withCookies(context.Background(), http.Cookie{Name: "accessToken", Value: token}),
		req,
	)
	require.NoError(t, err)
}

func TestMFAUnenroll_NoToken(t *testing.T) {
	client := authClient(t)
	_, err := client.MFAUnenroll(context.Background(), &authv1.MFAUnenrollRequest{})
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAUnenroll_NoMFA(t *testing.T) {
	client := authClient(t)
	req := &authv1.MFAUnenrollRequest{}
	_, err := client.MFAUnenroll(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeFailedPrecondition, connectErr.Code())
}

func TestMFAUnenroll_Success(t *testing.T) {
	// A throwaway user so unenrolling doesn't affect the shared mfaUserID.
	token := freshTestUser(t)
	tokenCookie := http.Cookie{Name: "accessToken", Value: token}

	mfaClient := mfaClient(t)
	factorID, secret := enrollFactor(t, mfaClient, tokenCookie)
	verifyReq := &authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	}
	verifyCtx := withCookies(context.Background(), tokenCookie)
	verifyInfo, _ := connect.CallInfoForClientContext(verifyCtx)
	verifyResp, err := mfaClient.MFAEnrollVerify(verifyCtx, verifyReq)
	require.NoError(t, err)

	// The pre-enrolment aal1 token no longer grants a session.
	client := authClient(t)
	staleReq := &authv1.MFAUnenrollRequest{}
	_, err = client.MFAUnenroll(withCookies(context.Background(), tokenCookie), staleReq)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	req := &authv1.MFAUnenrollRequest{
		Code: verifyResp.RecoveryCodes[0],
	}
	_, err = client.MFAUnenroll(
		withCookies(context.Background(), sessionCookie(t, verifyInfo.ResponseHeader())),
		req,
	)
	require.NoError(t, err)
}

func TestMFAEnroll_WithAccessToken(t *testing.T) {
	client := authClient(t)
	req := &authv1.MFAEnrollRequest{}
	resp, err := client.MFAEnroll(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.FactorId)
}

func TestGetCurrentUser_HasMFA_False(t *testing.T) {
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.False(t, resp.HasMfa)
}

func TestGetCurrentUser_HasMFA_True(t *testing.T) {
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(
		withCookies(context.Background(), mfaSessionToken),
		req,
	)
	require.NoError(t, err)
	assert.True(t, resp.HasMfa)
}

// The aal1 token issued before the MFA challenge is not a session.
func TestGetCurrentUser_PreMFAToken_Unauthenticated(t *testing.T) {
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	_, err := client.GetCurrentUser(withCookies(context.Background(), mfaAccessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestUpdatePassword_PreMFAToken_Unauthenticated(t *testing.T) {
	client := authClient(t)
	req := &authv1.UpdatePasswordRequest{
		NewPassword: "another-password", CurrentPassword: testUserPassword,
	}
	_, err := client.UpdatePassword(withCookies(context.Background(), mfaAccessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestUpdatePassword_WrongCurrentPassword_PermissionDenied(t *testing.T) {
	token := freshTestUser(t)
	client := authClient(t)
	req := &authv1.UpdatePasswordRequest{
		NewPassword: "newpassword123", CurrentPassword: "wrong",
	}
	_, err := client.UpdatePassword(
		withCookies(context.Background(), http.Cookie{Name: "accessToken", Value: token}),
		req,
	)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestUpdatePassword_TooShort_InvalidArgument(t *testing.T) {
	client := authClient(t)
	req := &authv1.UpdatePasswordRequest{
		NewPassword: "short", CurrentPassword: testUserPassword,
	}
	_, err := client.UpdatePassword(withCookies(context.Background(), accessToken), req)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestResetPassword_TooShort_InvalidArgument(t *testing.T) {
	client := authClient(t)
	_, err := client.ResetPassword(
		context.Background(),
		&authv1.ResetPasswordRequest{Token: "t", NewPassword: "short"},
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestMFAUnenroll_WrongCode_PermissionDenied(t *testing.T) {
	client := authClient(t)
	req := &authv1.MFAUnenrollRequest{Code: "000000"}
	_, err := client.MFAUnenroll(withCookies(context.Background(), mfaSessionToken), req)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}
