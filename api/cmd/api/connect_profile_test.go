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
		connect.NewRequest(&profilev1.GetProfileShareRequest{}),
	)
	require.Error(t, err)
}

func TestProfileShare_Lifecycle(t *testing.T) {
	ctx := context.Background()
	client := profileClient(t)

	// Start clean regardless of earlier tests.
	require.NoError(t, testApp.profileSharesRepo.Delete(ctx, testUserID))

	getReq := connect.NewRequest(&profilev1.GetProfileShareRequest{})
	setCookieOnRequest(getReq, accessToken)
	getResp, err := client.GetProfileShare(ctx, getReq)
	require.NoError(t, err)
	assert.Nil(t, getResp.Msg.Share, "no share should exist yet")

	createReq := connect.NewRequest(&profilev1.CreateProfileShareRequest{})
	setCookieOnRequest(createReq, accessToken)
	createResp, err := client.CreateProfileShare(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, createResp.Msg.Share)
	token := createResp.Msg.Share.Token
	assert.NotEmpty(t, token)
	_, err = time.Parse(time.RFC3339, createResp.Msg.Share.CreatedAt)
	assert.NoError(t, err)

	getReq = connect.NewRequest(&profilev1.GetProfileShareRequest{})
	setCookieOnRequest(getReq, accessToken)
	getResp, err = client.GetProfileShare(ctx, getReq)
	require.NoError(t, err)
	require.NotNil(t, getResp.Msg.Share)
	assert.Equal(t, token, getResp.Msg.Share.Token)

	// Regenerating replaces the token, invalidating the old link.
	createReq = connect.NewRequest(&profilev1.CreateProfileShareRequest{})
	setCookieOnRequest(createReq, accessToken)
	regenResp, err := client.CreateProfileShare(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, regenResp.Msg.Share)
	assert.NotEqual(t, token, regenResp.Msg.Share.Token)

	deleteReq := connect.NewRequest(&profilev1.DeleteProfileShareRequest{})
	setCookieOnRequest(deleteReq, accessToken)
	_, err = client.DeleteProfileShare(ctx, deleteReq)
	require.NoError(t, err)

	getReq = connect.NewRequest(&profilev1.GetProfileShareRequest{})
	setCookieOnRequest(getReq, accessToken)
	getResp, err = client.GetProfileShare(ctx, getReq)
	require.NoError(t, err)
	assert.Nil(t, getResp.Msg.Share, "share should be gone after delete")
}
