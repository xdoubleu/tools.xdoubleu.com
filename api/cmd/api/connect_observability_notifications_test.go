package main

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
)

func TestGetNotificationSettings_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	req := &observabilityv1.GetNotificationSettingsRequest{}
	resp, err := observabilityClient(
		t,
	).GetNotificationSettings(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)

	got := make(map[string]bool, len(resp.Settings))
	for _, s := range resp.Settings {
		got[s.SourceKey] = s.Enabled
	}
	assert.Contains(t, got, "unhealthy_feeds")
	assert.Contains(t, got, "open_feed_items")
	assert.Equal(t, testApp.config.NotifyEmailTo, resp.AdminEmail)
}

// Non-admins can read the toggles but not the admin's address.
func TestGetNotificationSettings_AsNonAdmin_Allowed(t *testing.T) {
	req := &observabilityv1.GetNotificationSettingsRequest{}
	resp, err := observabilityClient(
		t,
	).GetNotificationSettings(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Settings)
	assert.Empty(t, resp.AdminEmail)
}

// The settings are global, so only admins may change them.
func TestUpdateNotificationSettings_AsNonAdmin_Denied(t *testing.T) {
	req := &observabilityv1.UpdateNotificationSettingsRequest{
		SourceKey: "open_feed_items",
		Enabled:   false,
	}
	_, err := observabilityClient(
		t,
	).UpdateNotificationSettings(withCookies(context.Background(), accessToken), req)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestUpdateNotificationSettings_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	t.Cleanup(func() {
		req := &observabilityv1.UpdateNotificationSettingsRequest{
			SourceKey: "open_feed_items",
			Enabled:   true,
		}
		_, _ = observabilityClient(
			t,
		).UpdateNotificationSettings(withCookies(context.Background(), accessToken), req)
	})

	updateReq := &observabilityv1.UpdateNotificationSettingsRequest{
		SourceKey: "open_feed_items",
		Enabled:   false,
	}
	_, err := observabilityClient(
		t,
	).UpdateNotificationSettings(withCookies(context.Background(), accessToken), updateReq)
	require.NoError(t, err)

	getReq := &observabilityv1.GetNotificationSettingsRequest{}
	resp, err := observabilityClient(
		t,
	).GetNotificationSettings(withCookies(context.Background(), accessToken), getReq)
	require.NoError(t, err)

	for _, s := range resp.Settings {
		if s.SourceKey == "open_feed_items" {
			assert.False(t, s.Enabled)
		}
	}
}
