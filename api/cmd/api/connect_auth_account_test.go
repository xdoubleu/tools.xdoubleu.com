package main

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
)

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
	client := authClient(t)
	req := connect.NewRequest(
		&authv1.UpdatePasswordRequest{NewPassword: "newpassword123"},
	)
	setCookieOnRequest(req, accessToken)
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
	// "mfa-access" token maps to a user with a verified TOTP factor.
	client := authClient(t)
	req := connect.NewRequest(&authv1.MFAUnenrollRequest{})
	setCookieOnRequest(req, http.Cookie{Name: "accessToken", Value: "mfa-access"})
	_, err := client.MFAUnenroll(context.Background(), req)
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
	setCookieOnRequest(req, http.Cookie{Name: "accessToken", Value: "mfa-access"})
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Msg.HasMfa)
}
