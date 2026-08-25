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
	assert.Contains(t, got, "sentry_issues")
	assert.Contains(t, got, "failing_dependency_prs")
	assert.Contains(t, got, "unhealthy_feeds")
	assert.Equal(t, testApp.config.NotifyEmailTo, resp.Msg.AdminEmail)
}

// GetNotificationSettings/UpdateNotificationSettings are deliberately not
// admin-gated (issue #1228) — any authenticated user can see and toggle
// them, since the unhealthy-feeds toggle is surfaced from the feeds app.
func TestGetNotificationSettings_AsNonAdmin_Allowed(t *testing.T) {
	req := connect.NewRequest(&observabilityv1.GetNotificationSettingsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := observabilityClient(
		t,
	).GetNotificationSettings(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Settings)
}

func TestUpdateNotificationSettings_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	t.Cleanup(func() {
		req := connect.NewRequest(&observabilityv1.UpdateNotificationSettingsRequest{
			SourceKey: "sentry_issues",
			Enabled:   true,
		})
		setCookieOnRequest(req, accessToken)
		_, _ = observabilityClient(
			t,
		).UpdateNotificationSettings(context.Background(), req)
	})

	updateReq := connect.NewRequest(&observabilityv1.UpdateNotificationSettingsRequest{
		SourceKey: "sentry_issues",
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
		if s.SourceKey == "sentry_issues" {
			assert.False(t, s.Enabled)
		}
	}
}

func TestUpdateNotificationSettings_AsNonAdmin_Allowed(t *testing.T) {
	t.Cleanup(func() {
		req := connect.NewRequest(&observabilityv1.UpdateNotificationSettingsRequest{
			SourceKey: "unhealthy_feeds",
			Enabled:   true,
		})
		setCookieOnRequest(req, accessToken)
		_, _ = observabilityClient(
			t,
		).UpdateNotificationSettings(context.Background(), req)
	})

	req := connect.NewRequest(&observabilityv1.UpdateNotificationSettingsRequest{
		SourceKey: "unhealthy_feeds",
		Enabled:   false,
	})
	setCookieOnRequest(req, accessToken)
	_, err := observabilityClient(
		t,
	).UpdateNotificationSettings(context.Background(), req)
	require.NoError(t, err)
}
