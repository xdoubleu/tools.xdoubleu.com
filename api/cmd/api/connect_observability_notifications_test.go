package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
)

func TestGetNotificationSettings_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	req := connect.NewRequest(&observabilityv1.GetNotificationSettingsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(
		t,
	).GetNotificationSettings(context.Background(), req)
	require.NoError(t, err)

	got := make(map[string]bool, len(resp.Msg.Settings))
	for _, s := range resp.Msg.Settings {
		got[s.SourceKey] = s.Enabled
	}
	assert.Contains(t, got, "unhealthy_feeds")
	assert.Contains(t, got, "open_feed_items")
	assert.Equal(t, testApp.config.NotifyEmailTo, resp.Msg.AdminEmail)
}

// Non-admins can read the toggles but not the admin's address.
func TestGetNotificationSettings_AsNonAdmin_Allowed(t *testing.T) {
	req := connect.NewRequest(&observabilityv1.GetNotificationSettingsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(
		t,
	).GetNotificationSettings(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Settings)
	assert.Empty(t, resp.Msg.AdminEmail)
}

// The settings are global, so only admins may change them.
func TestUpdateNotificationSettings_AsNonAdmin_Denied(t *testing.T) {
	req := connect.NewRequest(&observabilityv1.UpdateNotificationSettingsRequest{
		SourceKey: "open_feed_items",
		Enabled:   false,
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(
		t,
	).UpdateNotificationSettings(context.Background(), req)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestUpdateNotificationSettings_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	t.Cleanup(func() {
		req := connect.NewRequest(&observabilityv1.UpdateNotificationSettingsRequest{
			SourceKey: "open_feed_items",
			Enabled:   true,
		})
		setCookieOnRequest(req, accessToken)
		_, _ = observabilityClient(
			t,
		).UpdateNotificationSettings(context.Background(), req)
	})

	updateReq := connect.NewRequest(&observabilityv1.UpdateNotificationSettingsRequest{
		SourceKey: "open_feed_items",
		Enabled:   false,
	})
	setCookieOnRequest(updateReq, accessToken)
	_, err := observabilityClient(
		t,
	).UpdateNotificationSettings(context.Background(), updateReq)
	require.NoError(t, err)

	getReq := connect.NewRequest(&observabilityv1.GetNotificationSettingsRequest{})
	setCookieOnRequest(getReq, accessToken)
	resp, err := observabilityClient(
		t,
	).GetNotificationSettings(context.Background(), getReq)
	require.NoError(t, err)

	for _, s := range resp.Msg.Settings {
		if s.SourceKey == "open_feed_items" {
			assert.False(t, s.Enabled)
		}
	}
}
