package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	account "tools.xdoubleu.com/internal/account"
	"tools.xdoubleu.com/internal/kit/testhelper"
)

func authClient(t *testing.T) authv1connect.AuthServiceClient {
	t.Helper()
	ts := connectServer(t)
	return authv1connect.NewAuthServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
		),
	)
}

func TestSignIn_Success(t *testing.T) {
	client := authClient(t)
	req := &authv1.SignInRequest{
		Email:      "valid@example.com",
		Password:   "password",
		RememberMe: false,
		Redirect:   "/",
	}
	resp, err := client.SignIn(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.NeedsMfa)
	assert.False(t, resp.EnrollMfa)
}

func TestSignIn_LockoutReturnsResourceExhausted(t *testing.T) {
	client := authClient(t)
	email := "lockout-" + uuid.NewString() + "@example.com"

	var err error
	for range 7 {
		_, err = client.SignIn(
			context.Background(),
			&authv1.SignInRequest{Email: email, Password: "wrong", Redirect: "/"},
		)
		require.Error(t, err)
	}
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
}

func TestSignIn_WithMFA(t *testing.T) {
	_, hasMFA := testApp.auth.HasVerifiedTOTP(
		context.Background(),
		mfaAccessToken.Value,
	)
	assert.True(t, hasMFA)
}

func TestSignIn_EmptyEmail(t *testing.T) {
	client := authClient(t)
	_, err := client.SignIn(
		context.Background(),
		&authv1.SignInRequest{
			Email:    "",
			Password: "password",
			Redirect: "/",
		},
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
		&authv1.SignInRequest{
			Email:    "valid@example.com",
			Password: "",
			Redirect: "/",
		},
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
		&authv1.SignInRequest{
			Email:    "valid@example.com",
			Password: "password",
			Redirect: "",
		},
	)
	require.NoError(t, err)
	assert.False(t, resp.NeedsMfa)
}

func TestSignIn_InvalidRedirect(t *testing.T) {
	client := authClient(t)
	_, err := client.SignIn(
		context.Background(),
		&authv1.SignInRequest{
			Email:    "valid@example.com",
			Password: "password",
			Redirect: "https://evil.com",
		},
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
		// Some browsers normalize a leading backslash into a protocol-relative URL.
		{"/\\evil.com", false},
		{"/\\/evil.com", false},
		{"\\/evil.com", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, account.IsRelativeURL(tt.url), "url=%q", tt.url)
	}
}

func TestSignIn_Success_UsesLaxCookies(t *testing.T) {
	// Session cookies must be SameSite=Lax so they attach on the cross-site
	// redirect from the OAuth AS to /oauth/consent.
	client := authClient(t)
	ctx, info := connect.NewClientContext(context.Background())
	_, err := client.SignIn(
		ctx,
		&authv1.SignInRequest{
			Email:      "valid@example.com",
			Password:   "password",
			RememberMe: true,
			Redirect:   "/",
		},
	)
	require.NoError(t, err)

	found := false
	for _, raw := range info.ResponseHeader().Values("Set-Cookie") {
		if strings.HasPrefix(raw, "accessToken=") {
			found = true
			assert.Contains(t, raw, "SameSite=Lax")
		}
	}
	assert.True(t, found, "expected an accessToken Set-Cookie header")
}

func TestForgotPassword_Success(t *testing.T) {
	client := authClient(t)
	_, err := client.ForgotPassword(
		context.Background(),
		&authv1.ForgotPasswordRequest{Email: "user@example.com"},
	)
	require.NoError(t, err)
}

func TestForgotPassword_EmptyEmail(t *testing.T) {
	client := authClient(t)
	_, err := client.ForgotPassword(
		context.Background(),
		&authv1.ForgotPasswordRequest{Email: ""},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestSignOut_Success(t *testing.T) {
	client := authClient(t)
	req := &authv1.SignOutRequest{}
	_, err := client.SignOut(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
}

func TestSignOut_NoToken(t *testing.T) {
	client := authClient(t)
	_, err := client.SignOut(
		context.Background(),
		&authv1.SignOutRequest{},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestGetCurrentUser_Success(t *testing.T) {
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Role)
}

func TestGetCurrentUser_NoToken(t *testing.T) {
	client := authClient(t)
	_, err := client.GetCurrentUser(
		context.Background(),
		&authv1.GetCurrentUserRequest{},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestGetCurrentUser_WithRefreshToken_Success(t *testing.T) {
	refresh := freshTestUserWithRefresh(t)
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(
		withCookies(context.Background(), http.Cookie{Name: "refreshToken", Value: refresh}),
		req,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Role)
}

func TestGetCurrentUser_NoAccessToken_FallsBackToRefreshToken(t *testing.T) {
	refresh := freshTestUserWithRefresh(t)
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(
		withCookies(context.Background(), http.Cookie{Name: "refreshToken", Value: refresh}),
		req,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Role)
}

func TestGetCurrentUser_ReturnsUserRole(t *testing.T) {
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Equal(t, "user", resp.Role)
}

func TestGetCurrentUser_ReturnsAdminRole(t *testing.T) {
	promoteToAdmin(t)
	defer demoteToUser(t)

	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Equal(t, "admin", resp.Role)
}

func TestGetCurrentUser_ReturnsEmptyAppAccess(t *testing.T) {
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Empty(t, resp.AppAccess)
}

func TestGetCurrentUser_ReturnsAppAccess_WithGrant(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	grantAppAccess(t, testUserID, "backlog")
	defer revokeAppAccess(t, testUserID, "backlog")

	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Contains(t, resp.AppAccess, "backlog")
}

func TestGetCurrentUser_ReturnsUserId(t *testing.T) {
	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Equal(t, testUserID, resp.UserId)
}

func TestGetCurrentUser_Admin_HasRole(t *testing.T) {
	promoteToAdmin(t)
	defer demoteToUser(t)

	client := authClient(t)
	req := &authv1.GetCurrentUserRequest{}
	resp, err := client.GetCurrentUser(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Equal(t, "admin", resp.Role)
	assert.IsType(t, []string{}, resp.AppAccess)
}

// mfaTokenCookie holds a real signed JWT for testUserID; MFA RPCs verify it,
// MFAEnrollSkip does not.
//
//nolint:gochecknoglobals // shared test fixture
var mfaTokenCookie = http.Cookie{
	Name:  "mfaToken",
	Value: "",
}

//nolint:gochecknoglobals // shared test fixture
var mfaRefreshTokenCookie = http.Cookie{
	Name:  "mfaRefreshToken",
	Value: "refresh",
}

func grantPinApps(t *testing.T, apps ...string) {
	t.Helper()
	for _, app := range apps {
		require.NoError(t, testApp.appUsersRepo.SetAppAccess(
			context.Background(), testUserID, app, true,
		))
		t.Cleanup(func() {
			_ = testApp.appUsersRepo.SetAppAccess(
				context.Background(), testUserID, app, false,
			)
		})
	}
}

func setTabPins(t *testing.T, pins []string) error {
	t.Helper()
	t.Cleanup(func() {
		// A nil list is stored as NULL: the user never chose.
		_ = testApp.appUsersRepo.SetTabPins(context.Background(), testUserID, nil)
	})
	_, err := authClient(t).SetTabPins(
		withCookies(context.Background(), accessToken),
		&authv1.SetTabPinsRequest{TabPins: pins},
	)
	return err
}

func currentUserPins(t *testing.T) *authv1.GetCurrentUserResponse {
	t.Helper()
	resp, err := authClient(t).GetCurrentUser(
		withCookies(context.Background(), accessToken),
		&authv1.GetCurrentUserRequest{},
	)
	require.NoError(t, err)
	return resp
}

func TestSetTabPins_DefaultsToNotCustomised(t *testing.T) {
	refresh := freshTestUserWithRefresh(t)

	resp, err := authClient(t).GetCurrentUser(
		withCookies(context.Background(), http.Cookie{Name: "refreshToken", Value: refresh}),
		&authv1.GetCurrentUserRequest{},
	)
	require.NoError(t, err)
	assert.False(t, resp.TabPinsCustomised)
	assert.Empty(t, resp.TabPins)
}

func TestSetTabPins_SavesInOrderAndReadsBack(t *testing.T) {
	grantPinApps(t, "books", "feeds")

	require.NoError(t, setTabPins(t, []string{"feeds", "books"}))

	resp := currentUserPins(t)
	assert.True(t, resp.TabPinsCustomised)
	assert.Equal(t, []string{"feeds", "books"}, resp.TabPins)
}

func TestSetTabPins_EmptyIsADeliberateNone(t *testing.T) {
	require.NoError(t, setTabPins(t, nil))

	resp := currentUserPins(t)
	assert.True(t, resp.TabPinsCustomised)
	assert.Empty(t, resp.TabPins)
}

func TestSetTabPins_Rejects(t *testing.T) {
	grantPinApps(t, "books")
	cases := map[string]struct {
		pins []string
		code connect.Code
	}{
		"more than three": {
			[]string{"books", "feeds", "games", "trains"}, connect.CodeInvalidArgument,
		},
		"duplicate": {[]string{"books", "books"}, connect.CodeInvalidArgument},
		"empty id":  {[]string{""}, connect.CodeInvalidArgument},
		"no access": {[]string{"factory-not-granted"}, connect.CodePermissionDenied},
		"mixed access": {
			[]string{"books", "factory-not-granted"},
			connect.CodePermissionDenied,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := setTabPins(t, tc.pins)
			require.Error(t, err)
			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tc.code, connectErr.Code())
		})
	}
}

func TestSetTabPins_AdminMayPinAnyApp(t *testing.T) {
	promoteToAdmin(t)
	defer demoteToUser(t)

	require.NoError(t, setTabPins(t, []string{"trains"}))
	assert.Equal(t, []string{"trains"}, currentUserPins(t).TabPins)
}

func TestSetTabPins_NoToken(t *testing.T) {
	_, err := authClient(t).SetTabPins(
		context.Background(), &authv1.SetTabPinsRequest{},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func recordAppOpen(t *testing.T, app string) error {
	t.Helper()
	_, err := authClient(t).RecordAppOpen(
		withCookies(context.Background(), accessToken),
		&authv1.RecordAppOpenRequest{App: app},
	)
	return err
}

func TestRecordAppOpen_CountsAnAppTheUserCanOpen(t *testing.T) {
	grantPinApps(t, "recipes")

	require.NoError(t, recordAppOpen(t, "recipes"))
}

func TestRecordAppOpen_Rejects(t *testing.T) {
	cases := map[string]struct {
		app  string
		code connect.Code
	}{
		"empty":          {"", connect.CodeInvalidArgument},
		"bad characters": {"Recipes/../x", connect.CodeInvalidArgument},
		"too long":       {strings.Repeat("a", 33), connect.CodeInvalidArgument},
		"no access":      {"factory-not-granted", connect.CodePermissionDenied},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := recordAppOpen(t, tc.app)
			require.Error(t, err)
			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tc.code, connectErr.Code())
		})
	}
}

func TestRecordAppOpen_NoToken(t *testing.T) {
	_, err := authClient(t).RecordAppOpen(
		context.Background(), &authv1.RecordAppOpenRequest{App: "recipes"},
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestGetCurrentUser_SuggestsTheMostOpenedApps(t *testing.T) {
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	_, err := pool.Exec(ctx, `
		INSERT INTO global.app_opens (user_id, day, app, count)
		VALUES ($1, CURRENT_DATE - 1, 'recipes', 7), ($1, CURRENT_DATE - 2, 'trains', 2)
		ON CONFLICT (user_id, day, app) DO UPDATE SET count = EXCLUDED.count`, testUserID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM global.app_opens WHERE user_id = $1`, testUserID)
	})

	resp := currentUserPins(t)

	assert.Equal(t, []string{"recipes", "trains"}, resp.SuggestedPins)
}

func TestGetCurrentUser_NoSuggestionsWithoutHistory(t *testing.T) {
	refresh := freshTestUserWithRefresh(t)
	resp, err := authClient(t).GetCurrentUser(
		withCookies(context.Background(), http.Cookie{Name: "refreshToken", Value: refresh}),
		&authv1.GetCurrentUserRequest{},
	)
	require.NoError(t, err)
	assert.Empty(t, resp.SuggestedPins)
}
