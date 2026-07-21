package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/models"
)

func clearOAuthConnections(t *testing.T) {
	t.Helper()
	_, err := testApp.db.Exec(t.Context(), "DELETE FROM global.oauth_connections")
	require.NoError(t, err)
}

func TestListOAuthConnections_AsAdmin_NoneConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Connections, 3)
	for _, c := range resp.Msg.Connections {
		assert.False(t, c.Connected)
		assert.Empty(t, c.ConnectedBy)
	}
}

func TestListOAuthConnections_AsAdmin_OneConnected(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "tok",
		},
		testUserID,
	))

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	require.NoError(t, err)

	for _, c := range resp.Msg.Connections {
		if c.Provider == string(models.OAuthProviderGithub) {
			assert.True(t, c.Connected)
			assert.NotEmpty(t, c.ConnectedAt)
		} else {
			assert.False(t, c.Connected)
		}
	}
}

func TestListOAuthConnections_NonAdmin(t *testing.T) {
	demoteToUser(t)

	req := connect.NewRequest(&observabilityv1.ListOAuthConnectionsRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(t).ListOAuthConnections(context.Background(), req)
	requirePermissionDenied(t, err)
}

func TestDisconnectOAuthConnection_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	clearOAuthConnections(t)

	require.NoError(t, testApp.oauthConnRepo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "tok",
		},
		testUserID,
	))

	req := connect.NewRequest(&observabilityv1.DisconnectOAuthConnectionRequest{
		Provider: string(models.OAuthProviderGithub),
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(
		t,
	).DisconnectOAuthConnection(context.Background(), req)
	require.NoError(t, err)

	_, _, err = testApp.oauthConnRepo.Get(t.Context(), models.OAuthProviderGithub)
	assert.Error(t, err, "connection should be gone after disconnect")
}

func TestDisconnectOAuthConnection_NonAdmin(t *testing.T) {
	demoteToUser(t)

	req := connect.NewRequest(&observabilityv1.DisconnectOAuthConnectionRequest{
		Provider: string(models.OAuthProviderGithub),
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(
		t,
	).DisconnectOAuthConnection(context.Background(), req)
	requirePermissionDenied(t, err)
}
