package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/database/postgres"
)

// NotificationSource keys the rows of global.notification_settings — one
// per email-notifying source shared by jobs.IssueNotifierJob and
// jobs.WeeklyDigestJob (issue #1214).
type NotificationSource string

const (
	NotificationSourceSentryIssues         NotificationSource = "sentry_issues"
	NotificationSourceFailingDependencyPRs NotificationSource = "failing_dependency_prs"
	NotificationSourceUnhealthyFeeds       NotificationSource = "unhealthy_feeds"
	NotificationSourceFailingMainCI        NotificationSource = "failing_main_ci"
	NotificationSourceSecurityAlerts       NotificationSource = "security_alerts"
)

// NotificationSetting is one row of global.notification_settings.
type NotificationSetting struct {
	SourceKey NotificationSource
	Enabled   bool
}

// NotificationSettingsRepository reads/writes per-source email notification
// toggles.
type NotificationSettingsRepository struct {
	db postgres.DB
}

func NewNotificationSettingsRepository(
	db postgres.DB,
) *NotificationSettingsRepository {
	return &NotificationSettingsRepository{db: db}
}

// List returns every notification source and its current enabled state.
func (r *NotificationSettingsRepository) List(
	ctx context.Context,
) ([]NotificationSetting, error) {
	rows, err := r.db.Query(
		ctx,
		"SELECT source_key, enabled FROM global.notification_settings ORDER BY source_key",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []NotificationSetting
	for rows.Next() {
		var s NotificationSetting
		if scanErr := rows.Scan(&s.SourceKey, &s.Enabled); scanErr != nil {
			return nil, scanErr
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

// IsEnabled reports whether source currently has email notifications
// enabled. An unknown source_key defaults to enabled, matching the seed
// data's default of on.
func (r *NotificationSettingsRepository) IsEnabled(
	ctx context.Context,
	source NotificationSource,
) (bool, error) {
	var enabled bool
	err := r.db.QueryRow(
		ctx,
		"SELECT enabled FROM global.notification_settings WHERE source_key = $1",
		source,
	).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	return enabled, nil
}

// SetEnabled updates a single source's enabled state.
func (r *NotificationSettingsRepository) SetEnabled(
	ctx context.Context,
	source NotificationSource,
	enabled bool,
) error {
	_, err := r.db.Exec(
		ctx,
		"UPDATE global.notification_settings SET enabled = $2 WHERE source_key = $1",
		source,
		enabled,
	)
	return err
}
