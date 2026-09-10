package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

// Channel modes for global.notification_channel_config.channel_mode — the
// single switch deciding where jobs.WeeklyDigestJob sends its alerts
// (issue #1482).
const (
	ChannelModeEmail = "email"
	ChannelModeSlack = "slack"
	ChannelModeBoth  = "both"
)

// defaultChannelMode is what Get falls back to when the config row is missing
// — matches the table default and the email-only behaviour that predates the
// Slack channel.
const defaultChannelMode = ChannelModeEmail

// IsValidChannelMode reports whether mode is one of the three accepted
// channel_mode values.
func IsValidChannelMode(mode string) bool {
	switch mode {
	case ChannelModeEmail, ChannelModeSlack, ChannelModeBoth:
		return true
	default:
		return false
	}
}

// NotificationChannelConfig is the decrypted single-row delivery-channel
// config. SlackWebhookURL is "" when no webhook has been configured.
type NotificationChannelConfig struct {
	ChannelMode     string
	SlackWebhookURL string
}

// NotificationChannelConfigRepository reads/writes the one-row
// global.notification_channel_config. The Slack webhook URL is encrypted at
// rest via sealer (same key as OAuth tokens); callers only ever see the
// decrypted value.
type NotificationChannelConfigRepository struct {
	db     postgres.DB
	sealer *crypto.Sealer
}

func NewNotificationChannelConfigRepository(
	db postgres.DB, sealer *crypto.Sealer,
) *NotificationChannelConfigRepository {
	return &NotificationChannelConfigRepository{db: db, sealer: sealer}
}

// Get returns the current channel mode and decrypted Slack webhook URL. A
// missing row (should not happen — the migration seeds it) degrades to
// email-only rather than erroring, so an alert is never lost to a config
// read.
func (r *NotificationChannelConfigRepository) Get(
	ctx context.Context,
) (NotificationChannelConfig, error) {
	var (
		mode    string
		webhook []byte
	)
	err := r.db.QueryRow(ctx, `
		SELECT channel_mode, slack_webhook_url
		FROM global.notification_channel_config
		WHERE id = TRUE
	`).Scan(&mode, &webhook)
	if errors.Is(err, pgx.ErrNoRows) {
		return NotificationChannelConfig{
			ChannelMode:     defaultChannelMode,
			SlackWebhookURL: "",
		}, nil
	}
	if err != nil {
		return NotificationChannelConfig{}, err
	}

	url, err := r.decryptWebhook(webhook)
	if err != nil {
		return NotificationChannelConfig{}, err
	}

	return NotificationChannelConfig{ChannelMode: mode, SlackWebhookURL: url}, nil
}

// SetChannelMode updates only the channel mode, leaving the webhook URL
// untouched. mode must be one of the ChannelMode* constants.
func (r *NotificationChannelConfigRepository) SetChannelMode(
	ctx context.Context, mode string,
) error {
	if !IsValidChannelMode(mode) {
		return fmt.Errorf("repositories: invalid channel_mode %q", mode)
	}
	_, err := r.db.Exec(ctx, `
		UPDATE global.notification_channel_config
		SET channel_mode = $1
		WHERE id = TRUE
	`, mode)
	return err
}

// SetSlackWebhookURL encrypts and stores url, or clears the stored webhook
// (stores NULL) when url is "".
func (r *NotificationChannelConfigRepository) SetSlackWebhookURL(
	ctx context.Context, url string,
) error {
	var stored []byte
	if url != "" {
		if r.sealer == nil {
			return ErrEncryptionNotConfigured
		}
		enc, err := r.sealer.Encrypt([]byte(url))
		if err != nil {
			return err
		}
		stored = enc
	}

	_, err := r.db.Exec(ctx, `
		UPDATE global.notification_channel_config
		SET slack_webhook_url = $1
		WHERE id = TRUE
	`, stored)
	return err
}

func (r *NotificationChannelConfigRepository) decryptWebhook(
	stored []byte,
) (string, error) {
	if len(stored) == 0 {
		return "", nil
	}
	if r.sealer == nil {
		return "", ErrEncryptionNotConfigured
	}
	plain, err := r.sealer.Decrypt(stored)
	if err != nil {
		return "", fmt.Errorf("%w: %w", models.ErrDecryptFailed, err)
	}
	return string(plain), nil
}
