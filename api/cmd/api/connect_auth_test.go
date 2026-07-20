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
	assert.False(t, resp.Msg.NeedsMfa)
	assert.False(t, resp.Msg.EnrollMfa)
}

func TestSignIn_WithMFA(t *testing.T) {
	// "mfa-access" is the mock token that has a verified TOTP factor.
	// HasVerifiedTOTP("mfa-access") returns true per the mock.
	factorID, hasMFA := testApp.auth.HasVerifiedTOTP(
		context.Background(),
		"mfa-access",
	)
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
	assert.False(t, resp.Msg.NeedsMfa)
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

func TestIsRelativeURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"/", true},
		{"/foo", true},
		{"/foo/bar?x=1", true},
		{"", false},
		{"https://evil.com", false},
		{"//evil.com", false},
		// Some browsers normalize a leading backslash to a slash, turning
		// these into protocol-relative URLs — see #449.
		{"/\\evil.com", false},
		{"/\\/evil.com", false},
		{"\\/evil.com", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, isRelativeURL(tt.url), "url=%q", tt.url)
	}
}

func TestSignIn_Success_UsesLaxCookies(t *testing.T) {
	// #445: session cookies must be SameSite=Lax so they still attach on the
	// cross-site redirect from Supabase to /oauth/consent.
	client := authClient(t)
	resp, err := client.SignIn(
		context.Background(),
		connect.NewRequest(&authv1.SignInRequest{
			Email:      "valid@example.com",
			Password:   "password",
			RememberMe: true,
			Redirect:   "/",
		}),
	)
	require.NoError(t, err)

	found := false
	for _, raw := range resp.Header().Values("Set-Cookie") {
		if strings.HasPrefix(raw, "accessToken=") {
			found = true
			assert.Contains(t, raw, "SameSite=Lax")
		}
	}
	assert.True(t, found, "expected an accessToken Set-Cookie header")
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
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Role)
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

func TestGetCurrentUser_WithRefreshToken_Success(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, http.Cookie{Name: "refreshToken", Value: "refresh"})
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Role)
}

func TestGetCurrentUser_NoAccessToken_FallsBackToRefreshToken(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(
		req,
		http.Cookie{Name: "refreshToken", Value: "refresh"},
	)
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Role)
}

func TestGetCurrentUser_ReturnsUserRole(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "user", resp.Msg.Role)
}

func TestGetCurrentUser_ReturnsAdminRole(t *testing.T) {
	promoteToAdmin(t)
	defer demoteToUser(t)

	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "admin", resp.Msg.Role)
}

func TestGetCurrentUser_ReturnsEmptyAppAccess(t *testing.T) {
	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.AppAccess)
}

func TestGetCurrentUser_ReturnsAppAccess_WithGrant(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	grantAppAccess(t, testUserID, "backlog")
	defer revokeAppAccess(t, testUserID, "backlog")

	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, resp.Msg.AppAccess, "backlog")
}

func TestGetCurrentUser_Admin_HasRole(t *testing.T) {
	promoteToAdmin(t)
	defer demoteToUser(t)

	client := authClient(t)
	req := connect.NewRequest(&authv1.GetCurrentUserRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetCurrentUser(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "admin", resp.Msg.Role)
	assert.IsType(t, []string{}, resp.Msg.AppAccess)
}

//nolint:gochecknoglobals // shared test fixture
var mfaTokenCookie = http.Cookie{
	Name:  "mfaToken",
	Value: "access",
}

//nolint:gochecknoglobals // shared test fixture
var mfaRefreshTokenCookie = http.Cookie{
	Name:  "mfaRefreshToken",
	Value: "refresh",
}
