package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	profilev1 "tools.xdoubleu.com/gen/profile/v1"
	"tools.xdoubleu.com/gen/profile/v1/profilev1connect"
)

func profileClient(t *testing.T) profilev1connect.ProfileServiceClient {
	t.Helper()
	ts := connectServer(t)
	return profilev1connect.NewProfileServiceClient(ts.Client(), ts.URL)
}

func TestGetProfileShare_Unauthenticated(t *testing.T) {
	client := profileClient(t)
	_, err := client.GetProfileShare(
		context.Background(),
		connect.NewRequest(&profilev1.GetProfileShareRequest{
			App: profilev1.ProfileApp_PROFILE_APP_READING,
		}),
	)
	require.Error(t, err)
}

func TestCreateProfileShare_RequiresDisplayName(t *testing.T) {
	ctx := context.Background()
	client := profileClient(t)

	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.appUsersRepo.SetDisplayName(ctx, testUserID, ""))
	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "reading"))

	req := connect.NewRequest(&profilev1.CreateProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_READING,
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.CreateProfileShare(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSetDisplayName(t *testing.T) {
	ctx := context.Background()
	client := profileClient(t)

	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))

	req := connect.NewRequest(&profilev1.SetDisplayNameRequest{DisplayName: "Alice"})
	setCookieOnRequest(req, accessToken)
	_, err := client.SetDisplayName(ctx, req)
	require.NoError(t, err)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, "Alice", user.DisplayName)
}

func TestSetDisplayName_Empty(t *testing.T) {
	ctx := context.Background()
	client := profileClient(t)

	req := connect.NewRequest(&profilev1.SetDisplayNameRequest{DisplayName: ""})
	setCookieOnRequest(req, accessToken)
	_, err := client.SetDisplayName(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestProfileShare_Lifecycle(t *testing.T) {
	ctx := context.Background()
	client := profileClient(t)

	// Start clean regardless of earlier tests, and ensure a display name is
	// set (required to create a share link).
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(
		t,
		testApp.appUsersRepo.SetDisplayName(ctx, testUserID, "Books Owner"),
	)
	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "reading"))

	getReq := connect.NewRequest(&profilev1.GetProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_READING,
	})
	setCookieOnRequest(getReq, accessToken)
	getResp, err := client.GetProfileShare(ctx, getReq)
	require.NoError(t, err)
	assert.Nil(t, getResp.Msg.Share, "no share should exist yet")

	createReq := connect.NewRequest(&profilev1.CreateProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_READING,
	})
	setCookieOnRequest(createReq, accessToken)
	createResp, err := client.CreateProfileShare(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, createResp.Msg.Share)
	token := createResp.Msg.Share.Token
	assert.NotEmpty(t, token)
	_, err = time.Parse(time.RFC3339, createResp.Msg.Share.CreatedAt)
	assert.NoError(t, err)

	getReq = connect.NewRequest(&profilev1.GetProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_READING,
	})
	setCookieOnRequest(getReq, accessToken)
	getResp, err = client.GetProfileShare(ctx, getReq)
	require.NoError(t, err)
	require.NotNil(t, getResp.Msg.Share)
	assert.Equal(t, token, getResp.Msg.Share.Token)

	// The games share is independent: creating/regenerating/deleting the
	// books link must not touch it.
	gamesCreateReq := connect.NewRequest(&profilev1.CreateProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_GAMES,
	})
	setCookieOnRequest(gamesCreateReq, accessToken)
	gamesCreateResp, err := client.CreateProfileShare(ctx, gamesCreateReq)
	require.NoError(t, err)
	gamesToken := gamesCreateResp.Msg.Share.Token

	// Regenerating replaces the token, invalidating the old link.
	createReq = connect.NewRequest(&profilev1.CreateProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_READING,
	})
	setCookieOnRequest(createReq, accessToken)
	regenResp, err := client.CreateProfileShare(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, regenResp.Msg.Share)
	assert.NotEqual(t, token, regenResp.Msg.Share.Token)

	deleteReq := connect.NewRequest(&profilev1.DeleteProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_READING,
	})
	setCookieOnRequest(deleteReq, accessToken)
	_, err = client.DeleteProfileShare(ctx, deleteReq)
	require.NoError(t, err)

	getReq = connect.NewRequest(&profilev1.GetProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_READING,
	})
	setCookieOnRequest(getReq, accessToken)
	getResp, err = client.GetProfileShare(ctx, getReq)
	require.NoError(t, err)
	assert.Nil(t, getResp.Msg.Share, "share should be gone after delete")

	// The games share survived the books deletion.
	gamesGetReq := connect.NewRequest(&profilev1.GetProfileShareRequest{
		App: profilev1.ProfileApp_PROFILE_APP_GAMES,
	})
	setCookieOnRequest(gamesGetReq, accessToken)
	gamesGetResp, err := client.GetProfileShare(ctx, gamesGetReq)
	require.NoError(t, err)
	require.NotNil(t, gamesGetResp.Msg.Share)
	assert.Equal(t, gamesToken, gamesGetResp.Msg.Share.Token)

	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID, "games"))
}
