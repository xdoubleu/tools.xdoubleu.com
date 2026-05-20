package main

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/internal/mocks"
)

func authClient(t *testing.T) authv1connect.AuthServiceClient {
	t.Helper()
	ts := connectServer(t)
	return authv1connect.NewAuthServiceClient(ts.Client(), ts.URL)
}

func TestSignIn_Success(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.SignInRequest{
		Email:      "valid@example.com",
		Password:   "password",
		RememberMe: false,
		Redirect:   "/",
	})
	resp, err := client.SignIn(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Msg.NeedsMfa)
	assert.True(t, resp.Msg.EnrollMfa)
}

func TestSignIn_WithMFA(t *testing.T) {
	// "mfa-access" is the mock token that has a verified TOTP factor.
	// HasVerifiedTOTP("mfa-access") returns true per the mock.
	factorID, hasMFA := testApp.services.Auth.HasVerifiedTOTP("mfa-access")
	assert.True(t, hasMFA)
	assert.Equal(t, mocks.MockedFactorID, factorID)
}

func TestSignIn_EmptyEmail(t *testing.T) {
	client := authClient(t)
	_, err := client.SignIn(
		context.Background(),
		connect.NewRequest(&authv1.SignInRequest{
			Email:    "",
			Password: "password",
			Redirect: "/",
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestSignIn_EmptyPassword(t *testing.T) {
	client := authClient(t)
	_, err := client.SignIn(
		context.Background(),
		connect.NewRequest(&authv1.SignInRequest{
			Email:    "valid@example.com",
			Password: "",
			Redirect: "/",
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestSignIn_EmptyRedirect(t *testing.T) {
	client := authClient(t)
	resp, err := client.SignIn(
		context.Background(),
		connect.NewRequest(&authv1.SignInRequest{
			Email:    "valid@example.com",
			Password: "password",
			Redirect: "",
		}),
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.NeedsMfa)
}

func TestSignIn_InvalidRedirect(t *testing.T) {
	client := authClient(t)
	_, err := client.SignIn(
		context.Background(),
		connect.NewRequest(&authv1.SignInRequest{
			Email:    "valid@example.com",
			Password: "password",
			Redirect: "https://evil.com",
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestForgotPassword_Success(t *testing.T) {
	client := authClient(t)
	_, err := client.ForgotPassword(context.Background(), connect.NewRequest(
		&authv1.ForgotPasswordRequest{Email: "user@example.com"},
	))
	require.NoError(t, err)
}

func TestForgotPassword_EmptyEmail(t *testing.T) {
	client := authClient(t)
	_, err := client.ForgotPassword(context.Background(), connect.NewRequest(
		&authv1.ForgotPasswordRequest{Email: ""},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestSignOut_Success(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.SignOutRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.SignOut(context.Background(), req)
	require.NoError(t, err)
}

func TestSignOut_NoToken(t *testing.T) {
	client := authClient(t)
	_, err := client.SignOut(
		context.Background(),
		connect.NewRequest(&authv1.SignOutRequest{}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestGetCurrentUser_Success(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
}

func TestGetCurrentUser_NoToken(t *testing.T) {
	client := authClient(t)
	_, err := client.GetCurrentUser(
		context.Background(),
		connect.NewRequest(&authv1.GetCurrentUserRequest{}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

//nolint:gochecknoglobals // shared test fixture
var mfaTokenCookie = http.Cookie{
	Name:  "mfaToken",
	Value: "access",
}
