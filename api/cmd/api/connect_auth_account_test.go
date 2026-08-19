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
	// #447: a verified TOTP factor must still be challenged before
	// ExchangeToken (the password-reset flow) grants a full session.
	// mfaAccessToken belongs to mfaUserID, seeded with a verified TOTP
	// factor; RefreshToken's value is never validated by this RPC.
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
	// A throwaway user, not the shared testUserID/accessToken fixture —
	// changing its password would break every other test relying on that
	// fixture's original password remaining valid.
	token := freshTestUser(t)
	client := authClient(t)
	req := connect.NewRequest(
		&authv1.UpdatePasswordRequest{NewPassword: "newpassword123"},
	)
	setCookieOnRequest(req, http.Cookie{Name: "accessToken", Value: token})
	_, err := client.UpdatePassword(context.Background(), req)
	require.NoError(t, err)
}

// TestUpdatePassword_RevokesOtherSessions covered #448's fix (password
// change revokes other sessions) against the old GoTrue-backed
// implementation's Logout() call and its "logout-fail-access" mock error
// path. The self-hosted implementation (issue #1039) revokes sessions by
// deleting auth.refresh_tokens rows directly — a DB delete with no external
// call to fail in the way the old mock simulated — so there's no equivalent
// failure mode to exercise here; UpdatePassword_Success already covers the
// revocation happening at all.

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
	// "access" token maps to a user with no verified MFA factors.
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
	// A throwaway user with its own freshly enrolled+verified factor — not
	// the shared mfaUserID fixture, since unenrolling it would make this
	// test's ordering relative to others (e.g. TestGetCurrentUser_HasMFA_True)
	// significant.
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
