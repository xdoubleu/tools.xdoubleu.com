package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"tools.xdoubleu.com/internal/database"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
)

const (
	// thresholdAlertRunEvery matches IssueNotifierJob's cadence.
	thresholdAlertRunEvery = 5 * time.Minute

	// sustainWindow is how long host_cpu_high/host_memory_high require
	// every sample to stay above threshold before the rule breaches, so a
	// single spike doesn't fire. host_disk_high deliberately doesn't use
	// this — a full disk is instant, not sustained.
	sustainWindow = 15 * time.Minute
	// instantLookback bounds how far back an "instant" rule (host_disk_high)
	// looks for the most recent host_metric_samples row — generous relative
	// to HostMetricsSnapshotJob's 60s scrape interval so a briefly-delayed
	// scrape doesn't read as "no data".
	instantLookback = 5 * time.Minute
	// ciStatsWindow is how far back ci_duration_high aggregates workflow
	// p95 durations from, matching the default window get_workflow_run_stats
	// uses when the caller doesn't specify one.
	ciStatsWindow = 30 * 24 * time.Hour

	hostCPUThresholdPercent    = 80.0
	hostMemoryThresholdPercent = 85.0
	hostDiskThresholdPercent   = 85.0
	// r2UsageThresholdBytes is a sane default, not a plan-derived figure —
	// there's no numeric threshold anywhere else in the codebase to inherit
	// from (issue #1283).
	r2UsageThresholdBytes = 50 * bytesPerGB
	// ciDurationThresholdMs flags a workflow whose p95 duration has grown
	// well past what any workflow in this repo normally takes.
	ciDurationThresholdMs = 15 * msPerMinute

	unitPercent = "%"
	unitBytes   = "bytes"
	unitMillis  = "ms"

	bytesPerGB  = 1024 * 1024 * 1024
	msPerMinute = 60_000
)

// hostMetricsSinceRepo is the subset of *repositories.HostMetricsRepository
// this job needs.
type hostMetricsSinceRepo interface {
	Since(ctx context.Context, since time.Time) ([]models.HostMetricSample, error)
}

// workflowDurationStatsRepo is the subset of
// *repositories.WorkflowRunsRepository this job needs.
type workflowDurationStatsRepo interface {
	WorkflowDurationStats(
		ctx context.Context, since time.Time,
	) ([]models.WorkflowDurationStat, error)
}

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

// ThresholdAlertJob evaluates a fixed set of threshold rules against stored
// samples (host metrics, R2 usage, CI duration history) and emails an admin
// on breach and on recovery, tracking state in global.alert_states so a
// rule re-arms after recovering — the behavior global.notified_issues'
// append-only dedup can't express (issue #1283).
type ThresholdAlertJob struct {
	rules         []alertRule
	settings      notificationSettingsRepo
	states        alertStateRepo
	notifications *notifications.Service
}

func NewThresholdAlertJob(
	hostMetrics hostMetricsSinceRepo,
	storage latestStorageSnapshotGetter,
	workflowRuns workflowDurationStatsRepo,
	transactionStats transactionStatsLister,
	settings notificationSettingsRepo,
	states alertStateRepo,
	notificationsSvc *notifications.Service,
) *ThresholdAlertJob {
	return &ThresholdAlertJob{
		rules: buildAlertRules(
			hostMetrics, storage, workflowRuns, transactionStats,
		),
		settings:      settings,
		states:        states,
		notifications: notificationsSvc,
	}
}

func buildAlertRules(
	hostMetrics hostMetricsSinceRepo,
	storage latestStorageSnapshotGetter,
	workflowRuns workflowDurationStatsRepo,
	transactionStats transactionStatsLister,
) []alertRule {
	return []alertRule{
		{
			key:       "host_cpu_high",
			label:     "Host CPU usage",
			source:    repositories.NotificationSourceHostCPUHigh,
			threshold: hostCPUThresholdPercent,
			unit:      unitPercent,
			evaluate: sustainedHostMetricEvaluator(
				hostMetrics, hostCPUThresholdPercent,
				func(s models.HostMetricSample) float64 { return s.CPUPercent },
			),
		},
		{
			key:       "host_memory_high",
			label:     "Host memory usage",
			source:    repositories.NotificationSourceHostMemoryHigh,
			threshold: hostMemoryThresholdPercent,
			unit:      unitPercent,
			evaluate: sustainedHostMetricEvaluator(
				hostMetrics, hostMemoryThresholdPercent,
				func(s models.HostMetricSample) float64 { return s.MemoryPercent },
			),
		},
		{
			key:       "host_disk_high",
			label:     "Host disk usage",
			source:    repositories.NotificationSourceHostDiskHigh,
			threshold: hostDiskThresholdPercent,
			unit:      unitPercent,
			evaluate: instantHostMetricEvaluator(
				hostMetrics, hostDiskThresholdPercent,
				func(s models.HostMetricSample) float64 { return s.DiskPercent },
			),
		},
		{
			key:       "r2_usage_high",
			label:     "R2 storage usage",
			source:    repositories.NotificationSourceR2UsageHigh,
			threshold: r2UsageThresholdBytes,
			unit:      unitBytes,
			evaluate:  r2UsageEvaluator(storage, r2UsageThresholdBytes),
		},
		{
			key:       "ci_duration_high",
			label:     "CI workflow duration (p95)",
			source:    repositories.NotificationSourceCIDurationHigh,
			threshold: ciDurationThresholdMs,
			unit:      unitMillis,
			evaluate:  ciDurationEvaluator(workflowRuns, ciDurationThresholdMs),
		},
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

// sustainedHostMetricEvaluator breaches only when every sample in the
// trailing sustainWindow is above threshold, so a single spike doesn't fire
// — "CPU above 80% for 15 minutes", not "CPU touched 80% once".
func sustainedHostMetricEvaluator(
	repo hostMetricsSinceRepo,
	threshold float64,
	extract func(models.HostMetricSample) float64,
) func(context.Context) (float64, bool, error) {
	return func(ctx context.Context) (float64, bool, error) {
		samples, err := repo.Since(ctx, time.Now().Add(-sustainWindow))
		if err != nil {
			return 0, false, err
		}
		if len(samples) == 0 {
			return 0, false, nil
		}

		sum := 0.0
		breaching := true
		for _, s := range samples {
			v := extract(s)
			sum += v
			if v <= threshold {
				breaching = false
			}
		}
		return sum / float64(len(samples)), breaching, nil
	}
}

// instantHostMetricEvaluator breaches based on the single most recent
// sample only — used for host_disk_high, where a full disk is an instant
// problem, not one that needs to sustain for 15 minutes to matter.
func instantHostMetricEvaluator(
	repo hostMetricsSinceRepo,
	threshold float64,
	extract func(models.HostMetricSample) float64,
) func(context.Context) (float64, bool, error) {
	return func(ctx context.Context) (float64, bool, error) {
		samples, err := repo.Since(ctx, time.Now().Add(-instantLookback))
		if err != nil {
			return 0, false, err
		}
		if len(samples) == 0 {
			return 0, false, nil
		}

		v := extract(samples[len(samples)-1])
		return v, v > threshold, nil
	}
}

// r2UsageEvaluator breaches when the latest storage snapshot's total size
// exceeds threshold bytes. No snapshot yet (the scan hasn't run) reads as
// not breaching rather than an error.
func r2UsageEvaluator(
	repo latestStorageSnapshotGetter,
	threshold float64,
) func(context.Context) (float64, bool, error) {
	return func(ctx context.Context) (float64, bool, error) {
		snap, err := repo.Latest(ctx)
		if errors.Is(err, database.ErrResourceNotFound) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		value := float64(snap.TotalSizeBytes)
		return value, value > threshold, nil
	}
}

// ciDurationEvaluator breaches when any workflow's p95 duration over
// ciStatsWindow exceeds threshold, reporting the highest p95 among them —
// one rule covering every workflow rather than a per-workflow rule set.
func ciDurationEvaluator(
	repo workflowDurationStatsRepo,
	threshold float64,
) func(context.Context) (float64, bool, error) {
	return func(ctx context.Context) (float64, bool, error) {
		stats, err := repo.WorkflowDurationStats(ctx, time.Now().Add(-ciStatsWindow))
		if err != nil {
			return 0, false, err
		}

		maxP95 := 0.0
		for _, s := range stats {
			if s.P95DurationMs > maxP95 {
				maxP95 = s.P95DurationMs
			}
		}
		return maxP95, maxP95 > threshold, nil
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
// get_alert_states never shows a stale reading.
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
// converting to friendlier units than the raw stored float.
func formatAlertValue(value float64, unit string) string {
	switch unit {
	case unitPercent:
		return fmt.Sprintf("%.1f%%", value)
	case unitBytes:
		return fmt.Sprintf("%.2f GB", value/bytesPerGB)
	case unitMillis:
		return fmt.Sprintf("%.1f min", value/msPerMinute)
	default:
		return fmt.Sprintf("%.2f", value)
	}
}
