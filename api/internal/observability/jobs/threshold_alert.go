package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
)

const (
	// thresholdAlertRunEvery matches IssueNotifierJob's cadence.
	thresholdAlertRunEvery = 5 * time.Minute

	unitMillis = "ms"

	msPerMinute = 60_000
)

// alertStateRepo is the subset of *repositories.AlertStatesRepository this
// job needs.
type alertStateRepo interface {
	Get(ctx context.Context, ruleKey string) (*models.AlertState, error)
	Upsert(ctx context.Context, s models.AlertState) error
}

// alertRule is one threshold rule: how to evaluate it, and how to describe
// it in a notification email. Rules are a typed Go slice rather than a
// database table — there is no admin CRUD UI to maintain, and rules stay
// unit-testable (issue #1283).
type alertRule struct {
	key       string
	label     string
	source    repositories.NotificationSource
	threshold float64
	unit      string
	// evaluate returns the rule's current value and whether it currently
	// breaches threshold.
	evaluate func(ctx context.Context) (value float64, breaching bool, err error)
}

// ThresholdAlertJob evaluates the slow-transaction threshold rules (the
// host/CI/storage rules it used to evaluate moved to Prometheus + Grafana
// alert rules, issue #1468 — see docs/adr-0011-slow-transaction-thresholds.md
// for why slow-transaction thresholds specifically did not) and emails an
// admin on breach and on recovery, tracking state in global.alert_states so a
// rule re-arms after recovering — the behavior global.notified_issues'
// append-only dedup can't express (issue #1283).
type ThresholdAlertJob struct {
	rules         []alertRule
	settings      notificationSettingsRepo
	states        alertStateRepo
	notifications *notifications.Service
}

func NewThresholdAlertJob(
	transactionStats transactionStatsLister,
	settings notificationSettingsRepo,
	states alertStateRepo,
	notificationsSvc *notifications.Service,
) *ThresholdAlertJob {
	return &ThresholdAlertJob{
		rules:         buildAlertRules(transactionStats),
		settings:      settings,
		states:        states,
		notifications: notificationsSvc,
	}
}

func buildAlertRules(
	transactionStats transactionStatsLister,
) []alertRule {
	return []alertRule{
		{
			key:       "slow_transaction_http_high",
			label:     "Slow HTTP handlers (p95)",
			source:    repositories.NotificationSourceSlowHTTPHigh,
			threshold: slowTransactionHTTPThresholdMs,
			unit:      unitMillis,
			evaluate: slowTransactionEvaluator(
				transactionStats,
				transactionClassHTTPHandler,
				slowTransactionHTTPThresholdMs,
			),
		},
		{
			key:       "slow_transaction_job_high",
			label:     "Slow background jobs (p95)",
			source:    repositories.NotificationSourceSlowJobHigh,
			threshold: slowTransactionJobThresholdMs,
			unit:      unitMillis,
			evaluate: slowTransactionEvaluator(
				transactionStats,
				transactionClassBackgroundJob,
				slowTransactionJobThresholdMs,
			),
		},
		{
			key:       "slow_transaction_frontend_high",
			label:     "Slow frontend transactions (p95)",
			source:    repositories.NotificationSourceSlowFEHigh,
			threshold: slowTransactionFrontendThresholdMs,
			unit:      unitMillis,
			evaluate: slowTransactionEvaluator(
				transactionStats,
				transactionClassFrontend,
				slowTransactionFrontendThresholdMs,
			),
		},
	}
}

func (j *ThresholdAlertJob) ID() string {
	return "threshold-alert"
}

func (j *ThresholdAlertJob) RunEvery() time.Duration {
	return thresholdAlertRunEvery
}

func (j *ThresholdAlertJob) Run(ctx context.Context, logger *slog.Logger) error {
	for _, rule := range j.rules {
		if err := j.evaluateRule(ctx, logger, rule); err != nil {
			return err
		}
	}
	return nil
}

// evaluateRule evaluates one rule and, on a breach/recovery transition,
// queues a notification email; the resulting state is only persisted once
// notifications confirms delivery (or the mailer is unconfigured, in which
// case the transition is retried on the next run), mirroring
// IssueNotifierJob.notifyOnce. When the rule's condition hasn't changed,
// the current value/threshold are still refreshed on every run so
// global.alert_states never shows a stale reading.
func (j *ThresholdAlertJob) evaluateRule(
	ctx context.Context,
	logger *slog.Logger,
	rule alertRule,
) error {
	enabled, err := j.settings.IsEnabled(ctx, rule.source)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	value, breaching, err := rule.evaluate(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "threshold-alert: evaluate failed",
			"rule", rule.key, essentialogger.ErrAttr(err))
		return nil
	}

	state, err := j.states.Get(ctx, rule.key)
	if err != nil {
		return err
	}
	wasBreaching := state != nil && state.Breaching

	if breaching == wasBreaching {
		var since, lastNotified *time.Time
		if state != nil {
			since = state.Since
			lastNotified = state.LastNotifiedAt
		}
		return j.states.Upsert(ctx, models.AlertState{
			RuleKey: rule.key, Breaching: breaching, Since: since,
			LastNotifiedAt: lastNotified, CurrentValue: value, Threshold: rule.threshold,
		})
	}

	return j.notifyTransition(rule, value, breaching)
}

// notifyTransition queues the breach/recovery email for a rule whose
// condition just changed. Emailing on recovery too (not just on breach) is
// deliberate: a breach email with no matching recovery email would leave
// the reader unsure whether the condition is still ongoing (issue #1283).
// It takes no context of its own -- the email is delivered asynchronously
// on notifications.Service's own worker, which supplies its callback a
// fresh one.
func (j *ThresholdAlertJob) notifyTransition(
	rule alertRule,
	value float64,
	breaching bool,
) error {
	var subject, body string
	var newSince *time.Time
	if breaching {
		now := time.Now()
		newSince = &now
		subject = fmt.Sprintf("[Alert] %s above threshold", rule.label)
		body = fmt.Sprintf(
			"%s is %s, above the %s threshold.",
			rule.label, formatAlertValue(value, rule.unit),
			formatAlertValue(rule.threshold, rule.unit),
		)
	} else {
		subject = fmt.Sprintf("[Alert] %s recovered", rule.label)
		body = fmt.Sprintf(
			"%s is back to %s, below the %s threshold.",
			rule.label, formatAlertValue(value, rule.unit),
			formatAlertValue(rule.threshold, rule.unit),
		)
	}

	j.notifications.Enqueue(
		subject,
		body,
		func(ctx context.Context, sendErr error) error {
			if errors.Is(sendErr, mailer.ErrNotConfigured) {
				return nil
			}
			if sendErr != nil {
				return sendErr
			}
			now := time.Now()
			return j.states.Upsert(ctx, models.AlertState{
				RuleKey: rule.key, Breaching: breaching, Since: newSince,
				LastNotifiedAt: &now, CurrentValue: value, Threshold: rule.threshold,
			})
		},
	)
	return nil
}

// formatAlertValue renders a rule's value/threshold in a notification body,
// converting to friendlier units than the raw stored float. Every remaining
// rule is millisecond-based (unitMillis) — the switch stays in case a
// non-millisecond rule returns.
func formatAlertValue(value float64, unit string) string {
	switch unit {
	case unitMillis:
		return fmt.Sprintf("%.1f min", value/msPerMinute)
	default:
		return fmt.Sprintf("%.2f", value)
	}
}
