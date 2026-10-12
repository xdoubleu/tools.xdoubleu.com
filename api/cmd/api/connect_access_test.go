package main

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessv1 "tools.xdoubleu.com/gen/access/v1"
	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/internal/models"
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
