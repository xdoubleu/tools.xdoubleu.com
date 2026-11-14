package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessv1 "tools.xdoubleu.com/gen/access/v1"
	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/zitadel"
)

func accessClient(t *testing.T) accessv1connect.AccessServiceClient {
	t.Helper()
	ts := connectServer(t)
	return accessv1connect.NewAccessServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
		),
	)
}

func TestAccessListUsers_NonAdmin(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	demoteToUser(t)

	client := accessClient(t)
	req := &accessv1.ListUsersRequest{}
	_, err := client.ListUsers(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestAccessListUsers_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := accessClient(t)
	req := &accessv1.ListUsersRequest{}
	resp, err := client.ListUsers(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Users)
}

func TestAccessListUsers_IncludesDisplayName(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(
		t,
		testApp.appUsersRepo.SetDisplayName(ctx, testUserID, "Ada Lovelace"),
	)
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	resp, err := accessClient(t).ListUsers(
		withCookies(context.Background(), accessToken),
		&accessv1.ListUsersRequest{},
	)
	require.NoError(t, err)
	var names []string
	for _, u := range resp.Users {
		names = append(names, u.DisplayName)
	}
	assert.Contains(t, names, "Ada Lovelace")
}

func TestAccessListUsers_Unauthenticated(t *testing.T) {
	client := accessClient(t)
	_, err := client.ListUsers(
		context.Background(),
		&accessv1.ListUsersRequest{},
	)
	require.Error(t, err)
}

func TestAccessSetRole_AsAdmin(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := accessClient(t)
	req := &accessv1.SetRoleRequest{
		UserId: testUserID,
		Role:   "user",
	}
	resp, err := client.SetRole(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.Equal(t, "user", resp.User.Role)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, models.RoleUser, user.Role)
}

func TestAccessSetRole_InvalidRole(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := accessClient(t)
	req := &accessv1.SetRoleRequest{
		UserId: testUserID,
		Role:   "superuser",
	}
	_, err := client.SetRole(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestAccessSetRole_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := accessClient(t)
	req := &accessv1.SetRoleRequest{UserId: testUserID, Role: "user"}
	_, err := client.SetRole(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestAccessSetAppAccess_GrantAndRevoke(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := accessClient(t)

	grantReq := &accessv1.SetAppAccessRequest{
		UserId:  testUserID,
		AppName: "backlog",
		Grant:   true,
	}
	_, err := client.SetAppAccess(withCookies(context.Background(), accessToken), grantReq)
	require.NoError(t, err)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Contains(t, user.AppAccess, "backlog")

	revokeReq := &accessv1.SetAppAccessRequest{
		UserId:  testUserID,
		AppName: "backlog",
		Grant:   false,
	}
	_, err = client.SetAppAccess(withCookies(context.Background(), accessToken), revokeReq)
	require.NoError(t, err)

	user2, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.NotContains(t, user2.AppAccess, "backlog")
}

func TestAccessSetAppAccess_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := accessClient(t)
	req := &accessv1.SetAppAccessRequest{
		UserId: testUserID, AppName: "backlog", Grant: true,
	}
	_, err := client.SetAppAccess(withCookies(context.Background(), accessToken), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestAccessSetDisplayName(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	resp, err := accessClient(t).SetDisplayName(
		withCookies(ctx, accessToken),
		&accessv1.SetDisplayNameRequest{UserId: testUserID, DisplayName: " Alice Doe "},
	)
	require.NoError(t, err)
	assert.Equal(t, "Alice Doe", resp.User.DisplayName)

	_, err = accessClient(t).SetDisplayName(
		withCookies(ctx, accessToken),
		&accessv1.SetDisplayNameRequest{UserId: testUserID, DisplayName: "  "},
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAccessSetDisplayName_NonAdmin(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	demoteToUser(t)

	_, err := accessClient(t).SetDisplayName(
		withCookies(ctx, accessToken),
		&accessv1.SetDisplayNameRequest{UserId: testUserID, DisplayName: "Alice"},
	)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

// fakeZitadelAPI answers user management's calls and records them.
type fakeZitadelAPI struct {
	mu        sync.Mutex
	calls     []string
	failOn    string
	userCount int
}

func (f *fakeZitadelAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := r.Method + " " + r.URL.Path
	f.calls = append(f.calls, key)
	if strings.HasSuffix(key, f.failOn) && f.failOn != "" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if key == "POST /v2/users/human" {
		f.userCount++
		_, _ = w.Write([]byte(`{"userId":"` + uuid.NewString() + `"}`))
		return
	}
	_, _ = w.Write([]byte(`{}`))
}

func useFakeZitadel(t *testing.T) *fakeZitadelAPI {
	t.Helper()
	fake := &fakeZitadelAPI{mu: sync.Mutex{}, calls: nil, failOn: "", userCount: 0}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	cfg := testhelper.NewTestConfig()
	cfg.OIDCIssuer, cfg.ZitadelServiceToken = srv.URL, "pat"
	testApp.auth.Directory = zitadel.New(cfg)
	t.Cleanup(func() { testApp.auth.Directory = nil })
	return fake
}

func TestAccessCreateAndDeactivateUser(t *testing.T) {
	ctx := withCookies(context.Background(), accessToken)
	fake := useFakeZitadel(t)
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	email := uuid.NewString() + "@example.com"

	created, err := accessClient(t).CreateUser(
		ctx, &accessv1.CreateUserRequest{Email: email, DisplayName: "New Person"},
	)
	require.NoError(t, err)
	assert.Equal(t, email, created.User.Email)
	assert.Equal(t, "New Person", created.User.DisplayName)
	assert.Equal(t, "user", created.User.Role)

	row, err := auth.NewRepository(testApp.db).GetUserByEmail(ctx, email)
	require.NoError(t, err)
	assert.NotEmpty(t, row.OIDCSubject)

	_, err = accessClient(t).CreateUser(ctx, &accessv1.CreateUserRequest{Email: email})
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))

	_, err = accessClient(t).DeactivateUser(
		ctx, &accessv1.DeactivateUserRequest{UserId: created.User.Id},
	)
	require.NoError(t, err)
	assert.Contains(t, fake.calls, "POST /v2/users/"+row.OIDCSubject+"/deactivate")

	row, err = auth.NewRepository(testApp.db).GetUserByEmail(ctx, email)
	require.NoError(t, err)
	assert.True(t, row.Deactivated)

	listed, err := accessClient(t).ListUsers(ctx, &accessv1.ListUsersRequest{})
	require.NoError(t, err)
	for _, u := range listed.Users {
		assert.NotEqual(t, created.User.Id, u.Id)
	}
}

func TestAccessCreateUser_FailedInviteLeavesNoOrphan(t *testing.T) {
	ctx := withCookies(context.Background(), accessToken)
	fake := useFakeZitadel(t)
	fake.failOn = "/invite_code"
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	email := uuid.NewString() + "@example.com"

	_, err := accessClient(t).CreateUser(ctx, &accessv1.CreateUserRequest{Email: email})
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))

	_, err = auth.NewRepository(testApp.db).GetUserByEmail(ctx, email)
	require.Error(t, err, "the local user is rolled back")
	var deletes int
	for _, call := range fake.calls {
		if strings.HasPrefix(call, "DELETE /v2/users/") {
			deletes++
		}
	}
	assert.Equal(t, 1, deletes, "the Zitadel user is rolled back")
}

func TestAccessCreateUser_Refusals(t *testing.T) {
	ctx := withCookies(context.Background(), accessToken)
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	demoteToUser(t)

	_, err := accessClient(t).CreateUser(ctx, &accessv1.CreateUserRequest{Email: "a@b.co"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = accessClient(t).DeactivateUser(
		ctx, &accessv1.DeactivateUserRequest{UserId: "x"},
	)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	_, err = accessClient(t).CreateUser(ctx, &accessv1.CreateUserRequest{Email: "a@b.co"})
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err), "no provider")

	useFakeZitadel(t)
	_, err = accessClient(t).CreateUser(ctx, &accessv1.CreateUserRequest{Email: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = accessClient(t).DeactivateUser(
		ctx, &accessv1.DeactivateUserRequest{UserId: testUserID},
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "not yourself")

	_, err = accessClient(t).DeactivateUser(
		ctx, &accessv1.DeactivateUserRequest{UserId: uuid.NewString()},
	)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
