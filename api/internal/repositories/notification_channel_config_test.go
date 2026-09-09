package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/repositories"
)

func resetChannelConfig(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), `
		UPDATE global.notification_channel_config
		SET channel_mode = 'email', slack_webhook_url = NULL
		WHERE id = TRUE
	`)
	require.NoError(t, err)
}

func TestNotificationChannelConfigDefault(t *testing.T) {
	resetChannelConfig(t)
	repo := repositories.NewNotificationChannelConfigRepository(testDB, testSealer(t))

	cfg, err := repo.Get(t.Context())
	require.NoError(t, err)
	assert.Equal(t, repositories.ChannelModeEmail, cfg.ChannelMode)
	assert.Empty(t, cfg.SlackWebhookURL)
}

func TestNotificationChannelConfigRoundTrip(t *testing.T) {
	resetChannelConfig(t)
	repo := repositories.NewNotificationChannelConfigRepository(testDB, testSealer(t))

	require.NoError(t, repo.SetChannelMode(t.Context(), repositories.ChannelModeBoth))
	require.NoError(t, repo.SetSlackWebhookURL(
		t.Context(), "https://hooks.slack.com/services/T/B/xyz",
	))

	cfg, err := repo.Get(t.Context())
	require.NoError(t, err)
	assert.Equal(t, repositories.ChannelModeBoth, cfg.ChannelMode)
	assert.Equal(t, "https://hooks.slack.com/services/T/B/xyz", cfg.SlackWebhookURL)

	// The URL is stored encrypted, not in cleartext.
	var raw []byte
	require.NoError(t, testDB.QueryRow(t.Context(), `
		SELECT slack_webhook_url FROM global.notification_channel_config WHERE id = TRUE
	`).Scan(&raw))
	assert.NotContains(t, string(raw), "hooks.slack.com")

	require.NoError(t, repo.SetSlackWebhookURL(t.Context(), ""))
	cfg, err = repo.Get(t.Context())
	require.NoError(t, err)
	assert.Empty(t, cfg.SlackWebhookURL)
}

func TestNotificationChannelConfigRejectsInvalidMode(t *testing.T) {
	resetChannelConfig(t)
	repo := repositories.NewNotificationChannelConfigRepository(testDB, testSealer(t))

	assert.Error(t, repo.SetChannelMode(t.Context(), "carrier-pigeon"))
}

func TestNotificationChannelConfigWithoutSealer(t *testing.T) {
	resetChannelConfig(t)
	repo := repositories.NewNotificationChannelConfigRepository(testDB, nil)

	err := repo.SetSlackWebhookURL(t.Context(), "https://hooks.slack.com/x")
	assert.ErrorIs(t, err, repositories.ErrEncryptionNotConfigured)
}
