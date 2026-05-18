package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminv1 "tools.xdoubleu.com/gen/admin/v1"
	"tools.xdoubleu.com/gen/admin/v1/adminv1connect"
	"tools.xdoubleu.com/internal/models"
)

func adminClient(t *testing.T) adminv1connect.AdminServiceClient {
	t.Helper()
	ts := connectServer(t)
	return adminv1connect.NewAdminServiceClient(ts.Client(), ts.URL)
}

func TestAdminListUsers_NonAdmin(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	demoteToUser(t)

	client := adminClient(t)
	req := connect.NewRequest(&adminv1.ListUsersRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.ListUsers(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestAdminListUsers_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := adminClient(t)
	req := connect.NewRequest(&adminv1.ListUsersRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.ListUsers(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Users)
}

func TestAdminListUsers_Unauthenticated(t *testing.T) {
	client := adminClient(t)
	_, err := client.ListUsers(
		context.Background(),
		connect.NewRequest(&adminv1.ListUsersRequest{}),
	)
	require.Error(t, err)
}

func TestAdminSetRole_AsAdmin(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := adminClient(t)
	req := connect.NewRequest(&adminv1.SetRoleRequest{
		UserId: testUserID,
		Role:   "user",
	})
	setCookieOnRequest(req, accessToken)
	resp, err := client.SetRole(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "user", resp.Msg.User.Role)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, models.RoleUser, user.Role)
}

func TestAdminSetRole_InvalidRole(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := adminClient(t)
	req := connect.NewRequest(&adminv1.SetRoleRequest{
		UserId: testUserID,
		Role:   "superuser",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.SetRole(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestAdminSetRole_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := adminClient(t)
	req := connect.NewRequest(&adminv1.SetRoleRequest{UserId: testUserID, Role: "user"})
	setCookieOnRequest(req, accessToken)
	_, err := client.SetRole(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestAdminSetAppAccess_GrantAndRevoke(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := adminClient(t)

	grantReq := connect.NewRequest(&adminv1.SetAppAccessRequest{
		UserId:  testUserID,
		AppName: "backlog",
		Grant:   true,
	})
	setCookieOnRequest(grantReq, accessToken)
	_, err := client.SetAppAccess(context.Background(), grantReq)
	require.NoError(t, err)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Contains(t, user.AppAccess, "backlog")

	revokeReq := connect.NewRequest(&adminv1.SetAppAccessRequest{
		UserId:  testUserID,
		AppName: "backlog",
		Grant:   false,
	})
	setCookieOnRequest(revokeReq, accessToken)
	_, err = client.SetAppAccess(context.Background(), revokeReq)
	require.NoError(t, err)

	user2, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.NotContains(t, user2.AppAccess, "backlog")
}

func TestAdminSetAppAccess_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := adminClient(t)
	req := connect.NewRequest(&adminv1.SetAppAccessRequest{
		UserId: testUserID, AppName: "backlog", Grant: true,
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.SetAppAccess(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}
