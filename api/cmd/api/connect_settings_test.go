package main

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	settingsv1 "tools.xdoubleu.com/gen/settings/v1"
	"tools.xdoubleu.com/gen/settings/v1/settingsv1connect"
)

func settingsClient(t *testing.T) settingsv1connect.SettingsServiceClient {
	t.Helper()
	ts := connectServer(t)
	return settingsv1connect.NewSettingsServiceClient(ts.Client(), ts.URL)
}

func TestGetSettings_Success(t *testing.T) {
	client := settingsClient(t)
	req := connect.NewRequest(&settingsv1.GetSettingsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetSettings(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Integrations)
}

func TestGetSettings_Unauthenticated(t *testing.T) {
	client := settingsClient(t)
	_, err := client.GetSettings(
		context.Background(),
		connect.NewRequest(&settingsv1.GetSettingsRequest{}),
	)
	require.Error(t, err)
}

func TestSaveSettings_Success(t *testing.T) {
	client := settingsClient(t)
	req := connect.NewRequest(&settingsv1.SaveSettingsRequest{
		Integrations: &settingsv1.Integrations{
			SteamApiKey:     "test-steam-key",
			SteamUserId:     "76561197960287930",
			HardcoverApiKey: "test-hardcover-key",
		},
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.SaveSettings(context.Background(), req)
	require.NoError(t, err)
}

func TestSaveSettings_InvalidSteamUserID(t *testing.T) {
	client := settingsClient(t)
	req := connect.NewRequest(&settingsv1.SaveSettingsRequest{
		Integrations: &settingsv1.Integrations{
			SteamUserId: "not-a-number",
		},
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.SaveSettings(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestSaveSettings_NilIntegrations(t *testing.T) {
	client := settingsClient(t)
	req := connect.NewRequest(&settingsv1.SaveSettingsRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.SaveSettings(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestSaveSettings_FieldsTooLong(t *testing.T) {
	client := settingsClient(t)
	req := connect.NewRequest(&settingsv1.SaveSettingsRequest{
		Integrations: &settingsv1.Integrations{
			SteamApiKey: strings.Repeat("x", steamAPIKeyMaxLen+1),
		},
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.SaveSettings(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestSaveSettings_RoundTrip(t *testing.T) {
	client := settingsClient(t)

	saveReq := connect.NewRequest(&settingsv1.SaveSettingsRequest{
		Integrations: &settingsv1.Integrations{
			SteamApiKey:     "round-trip-key",
			SteamUserId:     "76561197960287930",
			HardcoverApiKey: "round-trip-hc-key",
		},
	})
	setCookieOnRequest(saveReq, accessToken)
	_, err := client.SaveSettings(context.Background(), saveReq)
	require.NoError(t, err)

	getReq := connect.NewRequest(&settingsv1.GetSettingsRequest{})
	setCookieOnRequest(getReq, accessToken)
	resp, err := client.GetSettings(context.Background(), getReq)
	require.NoError(t, err)
	assert.Equal(t, "round-trip-key", resp.Msg.Integrations.SteamApiKey)
}
