package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
)

func dashboardClient(t *testing.T) dashboardv1connect.DashboardServiceClient {
	t.Helper()
	ts := connectServer(t)
	return dashboardv1connect.NewDashboardServiceClient(ts.Client(), ts.URL)
}

func TestGetDashboardShare_Unauthenticated(t *testing.T) {
	client := dashboardClient(t)
	_, err := client.GetDashboardShare(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetDashboardShareRequest{
			Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
		}),
	)
	require.Error(t, err)
}

func TestCreateDashboardShare_RequiresDisplayName(t *testing.T) {
	ctx := context.Background()
	client := dashboardClient(t)

	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.appUsersRepo.SetDisplayName(ctx, testUserID, ""))
	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "reading"))

	req := connect.NewRequest(&dashboardv1.CreateDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.CreateDashboardShare(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSetDisplayName(t *testing.T) {
	ctx := context.Background()
	client := dashboardClient(t)

	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))

	req := connect.NewRequest(&dashboardv1.SetDisplayNameRequest{DisplayName: "Alice"})
	setCookieOnRequest(req, accessToken)
	_, err := client.SetDisplayName(ctx, req)
	require.NoError(t, err)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, "Alice", user.DisplayName)
}

func TestSetDisplayName_Empty(t *testing.T) {
	ctx := context.Background()
	client := dashboardClient(t)

	req := connect.NewRequest(&dashboardv1.SetDisplayNameRequest{DisplayName: ""})
	setCookieOnRequest(req, accessToken)
	_, err := client.SetDisplayName(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestDashboardShare_Lifecycle(t *testing.T) {
	ctx := context.Background()
	client := dashboardClient(t)

	// Start clean regardless of earlier tests, and ensure a display name is
	// set (required to create a share link).
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(
		t,
		testApp.appUsersRepo.SetDisplayName(ctx, testUserID, "Reading Owner"),
	)
	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "reading"))

	getReq := connect.NewRequest(&dashboardv1.GetDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	})
	setCookieOnRequest(getReq, accessToken)
	getResp, err := client.GetDashboardShare(ctx, getReq)
	require.NoError(t, err)
	assert.Nil(t, getResp.Msg.Share, "no share should exist yet")

	createReq := connect.NewRequest(&dashboardv1.CreateDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	})
	setCookieOnRequest(createReq, accessToken)
	createResp, err := client.CreateDashboardShare(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, createResp.Msg.Share)
	token := createResp.Msg.Share.Token
	assert.NotEmpty(t, token)
	_, err = time.Parse(time.RFC3339, createResp.Msg.Share.CreatedAt)
	assert.NoError(t, err)

	getReq = connect.NewRequest(&dashboardv1.GetDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	})
	setCookieOnRequest(getReq, accessToken)
	getResp, err = client.GetDashboardShare(ctx, getReq)
	require.NoError(t, err)
	require.NotNil(t, getResp.Msg.Share)
	assert.Equal(t, token, getResp.Msg.Share.Token)

	// The games share is independent: creating/regenerating/deleting the
	// reading link must not touch it.
	gamesCreateReq := connect.NewRequest(&dashboardv1.CreateDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_GAMES,
	})
	setCookieOnRequest(gamesCreateReq, accessToken)
	gamesCreateResp, err := client.CreateDashboardShare(ctx, gamesCreateReq)
	require.NoError(t, err)
	gamesToken := gamesCreateResp.Msg.Share.Token

	// Regenerating replaces the token, invalidating the old link.
	createReq = connect.NewRequest(&dashboardv1.CreateDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	})
	setCookieOnRequest(createReq, accessToken)
	regenResp, err := client.CreateDashboardShare(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, regenResp.Msg.Share)
	assert.NotEqual(t, token, regenResp.Msg.Share.Token)

	deleteReq := connect.NewRequest(&dashboardv1.DeleteDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	})
	setCookieOnRequest(deleteReq, accessToken)
	_, err = client.DeleteDashboardShare(ctx, deleteReq)
	require.NoError(t, err)

	getReq = connect.NewRequest(&dashboardv1.GetDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	})
	setCookieOnRequest(getReq, accessToken)
	getResp, err = client.GetDashboardShare(ctx, getReq)
	require.NoError(t, err)
	assert.Nil(t, getResp.Msg.Share, "share should be gone after delete")

	// The games share survived the reading deletion.
	gamesGetReq := connect.NewRequest(&dashboardv1.GetDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_GAMES,
	})
	setCookieOnRequest(gamesGetReq, accessToken)
	gamesGetResp, err := client.GetDashboardShare(ctx, gamesGetReq)
	require.NoError(t, err)
	require.NotNil(t, gamesGetResp.Msg.Share)
	assert.Equal(t, gamesToken, gamesGetResp.Msg.Share.Token)

	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "games"))
}
