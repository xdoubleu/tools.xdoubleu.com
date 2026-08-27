// Package jobs holds background jobs that are cross-app observability
// concerns rather than scoped to a single app (see internal/observability).
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/github"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/sentryapi"
)

// runEvery matches the ~45s in-memory cache on the Sentry client with
// margin; "realtime" here means "within a few minutes", not sub-second.
const runEvery = 5 * time.Minute

// notifiedRepo is the subset of *repositories.NotifiedIssuesRepository this
// job needs.
type notifiedRepo interface {
	Exists(ctx context.Context, key string) (bool, error)
	Insert(ctx context.Context, key string) error
}

// failingPRLister is the subset of github.Client this job needs.
type failingPRLister interface {
	ListFailingPullRequests(ctx context.Context) ([]github.PullRequest, error)
}

// securityAlertLister is the subset of github.Client this job needs.
type securityAlertLister interface {
	ListSecurityAlerts(ctx context.Context) ([]github.SecurityAlert, error)
}

// latestStorageSnapshotGetter is the subset of
// *repositories.StorageSnapshotsRepository this job needs.
type latestStorageSnapshotGetter interface {
	Latest(ctx context.Context) (*models.StorageSnapshot, error)
}

// notificationSettingsRepo is the subset of
// *repositories.NotificationSettingsRepository this job needs.
type notificationSettingsRepo interface {
	IsEnabled(
		ctx context.Context,
		source repositories.NotificationSource,
	) (bool, error)
}

// IssueNotifierJob emails an admin (via notifications.Service) the first
// time a Sentry issue or a failing "dependencies"-labeled pull request
// (issue #915) is seen. Any provider being unconfigured degrades that part
// silently instead of failing the whole run, matching how the /monitoring
// dashboard already treats sentryapi.ErrNotConfigured/github.ErrNotConfigured.
// The actual email delivery is queued on notifications rather than sent
// inline, so a slow/rate-limited Resend call never blocks this job's worker
// (issue #923).
// issueNotifierGithubClient is the subset of github.Client this job needs,
// combining the failing-PR and security-alert lookups it notifies about.
type issueNotifierGithubClient interface {
	failingPRLister
	securityAlertLister
}

type IssueNotifierJob struct {
	sentry          sentryapi.Client
	gh              issueNotifierGithubClient
	notifications   *notifications.Service
	notified        notifiedRepo
	settings        notificationSettingsRepo
	storageSnapshot latestStorageSnapshotGetter
}

func NewIssueNotifierJob(
	sentry sentryapi.Client,
	gh issueNotifierGithubClient,
	notifications *notifications.Service,
	notified notifiedRepo,
	settings notificationSettingsRepo,
	storageSnapshot latestStorageSnapshotGetter,
) *IssueNotifierJob {
	return &IssueNotifierJob{
		sentry:          sentry,
		gh:              gh,
		notifications:   notifications,
		notified:        notified,
		settings:        settings,
		storageSnapshot: storageSnapshot,
	}
}

func (j *IssueNotifierJob) ID() string {
	return "notify-new-issues"
}

func (j *IssueNotifierJob) RunEvery() time.Duration {
	return runEvery
}

func (j *IssueNotifierJob) Run(ctx context.Context, logger *slog.Logger) error {
	if err := j.notifySentry(ctx, logger); err != nil {
		return err
	}
	if err := j.notifyGithub(ctx, logger); err != nil {
		return err
	}
	if err := j.notifySecurityAlerts(ctx, logger); err != nil {
		return err
	}
	return j.notifyOrphans(ctx, logger)
}

func (j *IssueNotifierJob) notifySentry(
	ctx context.Context,
	logger *slog.Logger,
) error {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceSentryIssues,
	)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	issues, err := j.sentry.ListUnresolvedIssues(ctx)
	if errors.Is(err, sentryapi.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		logAPIErr(ctx, logger, "issue-notifier: failed to list sentry issues",
			err, sentryapi.IsTransientAPIError(err))
		return nil
	}

	for _, issue := range issues {
		key := "sentry:" + issue.ID
		subject := fmt.Sprintf("[Sentry] %s", issue.Title)
		body := fmt.Sprintf(
			"Level: %s\nProject: %s\nCulprit: %s\n\n%s",
			issue.Level, issue.Project, issue.Culprit, issue.Permalink,
		)
		if err = j.notifyOnce(ctx, key, subject, body); err != nil {
			return err
		}
	}
	return nil
}

// notifyGithub emails an admin the first time a "dependencies"-labeled pull
// request is seen with a failing CI check on its current head commit. The
// dedup key includes the head SHA (not just the PR number) so a re-push —
// Renovate rebasing, or a fix landing and then failing differently — is
// notified again, mirroring how notifySentry keys on issue ID rather than a
// static "there's a new issue" key.
func (j *IssueNotifierJob) notifyGithub(
	ctx context.Context,
	logger *slog.Logger,
) error {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceFailingDependencyPRs,
	)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	prs, err := j.gh.ListFailingPullRequests(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		logAPIErr(ctx, logger, "issue-notifier: failed to list failing pull requests",
			err, github.IsTransientAPIError(err))
		return nil
	}

	for _, pr := range prs {
		key := fmt.Sprintf("github:pr:%d:%s", pr.Number, pr.HeadSHA)
		subject := fmt.Sprintf("[GitHub] Dependency PR #%d failing CI", pr.Number)
		body := fmt.Sprintf(
			"%s\n%s\n\nFailing checks: %s",
			pr.Title, pr.URL, failingCheckNames(pr.FailingChecks),
		)
		if err = j.notifyOnce(ctx, key, subject, body); err != nil {
			return err
		}
	}
	return nil
}

// notifySecurityAlerts emails an admin the first time a Dependabot,
// code-scanning, or secret-scanning alert is seen. The dedup key includes
// the alert type (not just the number) since alert numbers aren't unique
// across the three alert sources.
func (j *IssueNotifierJob) notifySecurityAlerts(
	ctx context.Context,
	logger *slog.Logger,
) error {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceSecurityAlerts,
	)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	alerts, err := j.gh.ListSecurityAlerts(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		logAPIErr(ctx, logger, "issue-notifier: failed to list security alerts",
			err, github.IsTransientAPIError(err))
		return nil
	}

	for _, alert := range alerts {
		key := fmt.Sprintf("security:%s:%d", alert.Type, alert.Number)
		subject := fmt.Sprintf("[GitHub] %s alert #%d", alert.Type, alert.Number)
		body := fmt.Sprintf(
			"Severity: %s\n%s\n\n%s",
			alert.Severity, alert.Summary, alert.URL,
		)
		if err = j.notifyOnce(ctx, key, subject, body); err != nil {
			return err
		}
	}
	return nil
}

// notifyOrphans emails an admin the first time an orphaned R2 storage object
// (books app's daily StorageScanJob, see storage_scan.go) is seen in the
// latest snapshot. Keyed on the storage key so a still-orphaned object never
// re-notifies, but one deleted-then-re-orphaned by coincidence would.
//
// snap.OrphanKeys is a capped sample (maxOrphanKeys in storage_scan.go) — an
// orphan count beyond that cap still shows up in OrphanCount everywhere else
// (get_storage_stats, the Issues page), it just won't get its own email
// until it's within the sampled set.
func (j *IssueNotifierJob) notifyOrphans(
	ctx context.Context,
	logger *slog.Logger,
) error {
	enabled, err := j.settings.IsEnabled(
		ctx,
		repositories.NotificationSourceOrphanedStorage,
	)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	snap, err := j.storageSnapshot.Latest(ctx)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil
	}
	if err != nil {
		logger.ErrorContext(ctx, "issue-notifier: failed to load storage snapshot",
			essentialogger.ErrAttr(err))
		return nil
	}

	for _, key := range snap.OrphanKeys {
		notifyKey := "orphan:" + key
		subject := fmt.Sprintf("[Storage] Orphaned object %s", key)
		body := fmt.Sprintf(
			"Orphaned R2 object with no matching book_files row.\n\n"+
				"Key: %s\nDetected as of scan: %s",
			key, snap.ScannedAt.Format(time.RFC3339),
		)
		if err = j.notifyOnce(ctx, notifyKey, subject, body); err != nil {
			return err
		}
	}
	return nil
}

// failingCheckNames renders a pull request's failing checks as a
// comma-separated "name (conclusion)" list for the notification body.
func failingCheckNames(checks []github.FailingCheck) string {
	names := make([]string, len(checks))
	for i, c := range checks {
		names[i] = fmt.Sprintf("%s (%s)", c.Name, c.Conclusion)
	}
	return strings.Join(names, ", ")
}

// logAPIErr logs a poll failure at Warn (transient, self-heals on the next
// 5-minute poll) or Error (reaches Sentry, needs a look) depending on
// whether the client classified the error as a known-benign shape.
func logAPIErr(
	ctx context.Context, logger *slog.Logger, msg string, err error, transient bool,
) {
	if transient {
		logger.WarnContext(ctx, msg, essentialogger.ErrAttr(err))
		return
	}
	logger.ErrorContext(ctx, msg, essentialogger.ErrAttr(err))
}

// notifyOnce queues subject/body for delivery and records key as notified,
// unless key was already notified. The dedup key is only inserted once
// notifications confirms a successful send, so a failed send is retried on
// the next run instead of being silently dropped.
func (j *IssueNotifierJob) notifyOnce(
	ctx context.Context,
	key, subject, body string,
) error {
	exists, err := j.notified.Exists(ctx, key)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	j.notifications.Enqueue(subject, body, func(ctx context.Context, err error) error {
		if errors.Is(err, mailer.ErrNotConfigured) {
			return nil
		}
		if err != nil {
			return err
		}
		return j.notified.Insert(ctx, key)
	})
	return nil
}
