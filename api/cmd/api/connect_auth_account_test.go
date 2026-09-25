package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
)

func TestExchangeToken_Success(t *testing.T) {
	client := authClient(t)
	resp, err := client.ExchangeToken(context.Background(), connect.NewRequest(
		&authv1.ExchangeTokenRequest{
			AccessToken: accessToken.Value, RefreshToken: "refresh",
		},
	))
	require.NoError(t, err)
	assert.NotNil(t, resp)
	setCookieHeaders := resp.Header().Values("Set-Cookie")
	assert.NotEmpty(t, setCookieHeaders)
}

func TestExchangeToken_NeedsMFA(t *testing.T) {
	// A verified TOTP factor must still be challenged before ExchangeToken grants
	// a session. mfaAccessToken's user has one.
	client := authClient(t)
	resp, err := client.ExchangeToken(context.Background(), connect.NewRequest(
		&authv1.ExchangeTokenRequest{
			AccessToken:  mfaAccessToken.Value,
			RefreshToken: "mfa-refresh",
		},
	))
	require.NoError(t, err)
	assert.True(t, resp.Msg.NeedsMfa)

	setCookieHeaders := resp.Header().Values("Set-Cookie")
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
	_, err := client.ExchangeToken(context.Background(), connect.NewRequest(
		&authv1.ExchangeTokenRequest{
			AccessToken:  "bad-token",
			RefreshToken: "bad-refresh",
		},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestExchangeToken_EmptyAccessToken(t *testing.T) {
	client := authClient(t)
	_, err := client.ExchangeToken(context.Background(), connect.NewRequest(
		&authv1.ExchangeTokenRequest{AccessToken: "", RefreshToken: "refresh"},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestExchangeToken_EmptyRefreshToken(t *testing.T) {
	client := authClient(t)
	_, err := client.ExchangeToken(context.Background(), connect.NewRequest(
		&authv1.ExchangeTokenRequest{AccessToken: "access", RefreshToken: ""},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestUpdatePassword_NoToken(t *testing.T) {
	client := authClient(t)
	_, err := client.UpdatePassword(context.Background(), connect.NewRequest(
		&authv1.UpdatePasswordRequest{NewPassword: "newpass"},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestUpdatePassword_EmptyPassword(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.UpdatePasswordRequest{NewPassword: ""})
	setCookieOnRequest(req, accessToken)
	_, err := client.UpdatePassword(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestUpdatePassword_Success(t *testing.T) {
	// A throwaway user, so changing its password doesn't break shared fixtures.
	token := freshTestUser(t)
	client := authClient(t)
	req := connect.NewRequest(
		&authv1.UpdatePasswordRequest{NewPassword: "newpassword123"},
	)
	setCookieOnRequest(req, http.Cookie{Name: "accessToken", Value: token})
	_, err := client.UpdatePassword(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAUnenroll_NoToken(t *testing.T) {
	client := authClient(t)
	_, err := client.MFAUnenroll(context.Background(), connect.NewRequest(
		&authv1.MFAUnenrollRequest{},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestMFAUnenroll_NoMFA(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.MFAUnenrollRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.MFAUnenroll(context.Background(), req)
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
	verifyReq := connect.NewRequest(&authv1.MFAEnrollVerifyRequest{
		FactorId: factorID,
		Code:     currentTOTPCode(t, secret),
	})
	setCookieOnRequest(verifyReq, tokenCookie)
	_, err := mfaClient.MFAEnrollVerify(context.Background(), verifyReq)
	require.NoError(t, err)

	client := authClient(t)
	req := connect.NewRequest(&authv1.MFAUnenrollRequest{})
	setCookieOnRequest(req, tokenCookie)
	_, err = client.MFAUnenroll(context.Background(), req)
	require.NoError(t, err)
}

func TestMFAEnroll_WithAccessToken(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.MFAEnrollRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.MFAEnroll(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.FactorId)
}

func TestGetCurrentUser_HasMFA_False(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Msg.HasMfa)
}

func TestGetCurrentUser_HasMFA_True(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, mfaAccessToken)
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Msg.HasMfa)
}
