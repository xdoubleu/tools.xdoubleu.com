package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
)

func dashboardClient(t *testing.T) dashboardv1connect.DashboardServiceClient {
	t.Helper()
	ts := connectServer(t)
	return dashboardv1connect.NewDashboardServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
		),
	)
}

func TestGetDashboardShare_Unauthenticated(t *testing.T) {
	client := dashboardClient(t)
	_, err := client.GetDashboardShare(
		context.Background(),
		&dashboardv1.GetDashboardShareRequest{
			Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
		},
	)
	require.Error(t, err)
}

func TestCreateDashboardShare_RequiresDisplayName(t *testing.T) {
	ctx := context.Background()
	client := dashboardClient(t)

	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.appUsersRepo.SetDisplayName(ctx, testUserID, ""))
	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "reading"))

	req := &dashboardv1.CreateDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	}
	_, err := client.CreateDashboardShare(withCookies(ctx, accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSetDisplayName(t *testing.T) {
	ctx := context.Background()
	client := dashboardClient(t)

	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))

	req := &dashboardv1.SetDisplayNameRequest{DisplayName: "Alice"}
	_, err := client.SetDisplayName(withCookies(ctx, accessToken), req)
	require.NoError(t, err)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, "Alice", user.DisplayName)
}

func TestSetDisplayName_Empty(t *testing.T) {
	ctx := context.Background()
	client := dashboardClient(t)

	req := &dashboardv1.SetDisplayNameRequest{DisplayName: ""}
	_, err := client.SetDisplayName(withCookies(ctx, accessToken), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestDashboardShare_Lifecycle(t *testing.T) {
	ctx := context.Background()
	client := dashboardClient(t)

	// Start clean; a display name is required to create a share link.
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(
		t,
		testApp.appUsersRepo.SetDisplayName(ctx, testUserID, "Reading Owner"),
	)
	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "reading"))

	getReq := &dashboardv1.GetDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	}
	getResp, err := client.GetDashboardShare(withCookies(ctx, accessToken), getReq)
	require.NoError(t, err)
	assert.Nil(t, getResp.Share, "no share should exist yet")

	createReq := &dashboardv1.CreateDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	}
	createResp, err := client.CreateDashboardShare(
		withCookies(ctx, accessToken),
		createReq,
	)
	require.NoError(t, err)
	require.NotNil(t, createResp.Share)
	token := createResp.Share.Token
	assert.NotEmpty(t, token)
	_, err = time.Parse(time.RFC3339, createResp.Share.CreatedAt)
	assert.NoError(t, err)

	getReq = &dashboardv1.GetDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	}
	getResp, err = client.GetDashboardShare(withCookies(ctx, accessToken), getReq)
	require.NoError(t, err)
	require.NotNil(t, getResp.Share)
	assert.Equal(t, token, getResp.Share.Token)

	// The games share is independent of the reading link.
	gamesCreateReq := &dashboardv1.CreateDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_GAMES,
	}
	gamesCreateResp, err := client.CreateDashboardShare(
		withCookies(ctx, accessToken),
		gamesCreateReq,
	)
	require.NoError(t, err)
	gamesToken := gamesCreateResp.Share.Token

	createReq = &dashboardv1.CreateDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	}
	regenResp, err := client.CreateDashboardShare(withCookies(ctx, accessToken), createReq)
	require.NoError(t, err)
	require.NotNil(t, regenResp.Share)
	assert.NotEqual(t, token, regenResp.Share.Token)

	deleteReq := &dashboardv1.DeleteDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	}
	_, err = client.DeleteDashboardShare(withCookies(ctx, accessToken), deleteReq)
	require.NoError(t, err)

	getReq = &dashboardv1.GetDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_READING,
	}
	getResp, err = client.GetDashboardShare(withCookies(ctx, accessToken), getReq)
	require.NoError(t, err)
	assert.Nil(t, getResp.Share, "share should be gone after delete")

	gamesGetReq := &dashboardv1.GetDashboardShareRequest{
		Kind: dashboardv1.DashboardKind_DASHBOARD_KIND_GAMES,
	}
	gamesGetResp, err := client.GetDashboardShare(
		withCookies(ctx, accessToken),
		gamesGetReq,
	)
	require.NoError(t, err)
	require.NotNil(t, gamesGetResp.Share)
	assert.Equal(t, gamesToken, gamesGetResp.Share.Token)

	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "games"))
}
