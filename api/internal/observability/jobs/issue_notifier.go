// Package jobs holds background jobs that are cross-app observability
// concerns rather than scoped to a single app (see internal/observability).
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/sentryapi"
)

// deploymentErrorPhase is the DigitalOcean deployment phase treated as the
// DO equivalent of a "new issue" — there is no issue tracker on that side,
// so a failed deployment is what DeployCard.tsx already flags as an error
// state.
const deploymentErrorPhase = "ERROR"

// runEvery matches the ~45s in-memory cache on the Sentry/DigitalOcean
// clients with margin; "realtime" here means "within a few minutes", not
// sub-second.
const runEvery = 5 * time.Minute

// notifiedRepo is the subset of *repositories.NotifiedIssuesRepository this
// job needs.
type notifiedRepo interface {
	Exists(ctx context.Context, key string) (bool, error)
	Insert(ctx context.Context, key string) error
}

// IssueNotifierJob emails an admin (via mailer.Client) the first time a
// Sentry issue or a failed DigitalOcean deployment is seen (issue #561).
// Either provider being unconfigured degrades that half silently instead of
// failing the whole run, matching how the /monitoring dashboard already
// treats sentryapi.ErrNotConfigured/digitalocean.ErrNotConfigured.
type IssueNotifierJob struct {
	sentry   sentryapi.Client
	do       digitalocean.Client
	mail     mailer.Client
	notified notifiedRepo
}

func NewIssueNotifierJob(
	sentry sentryapi.Client,
	do digitalocean.Client,
	mail mailer.Client,
	notified notifiedRepo,
) *IssueNotifierJob {
	return &IssueNotifierJob{
		sentry:   sentry,
		do:       do,
		mail:     mail,
		notified: notified,
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
	return j.notifyDigitalOcean(ctx, logger)
}

func (j *IssueNotifierJob) notifySentry(
	ctx context.Context,
	logger *slog.Logger,
) error {
	issues, err := j.sentry.ListUnresolvedIssues(ctx)
	if errors.Is(err, sentryapi.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		logger.ErrorContext(ctx, "issue-notifier: failed to list sentry issues",
			essentialogger.ErrAttr(err))
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

func (j *IssueNotifierJob) notifyDigitalOcean(
	ctx context.Context,
	logger *slog.Logger,
) error {
	deployment, err := j.do.LatestDeployment(ctx)
	if errors.Is(err, digitalocean.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		logger.ErrorContext(ctx, "issue-notifier: failed to get latest deployment",
			essentialogger.ErrAttr(err))
		return nil
	}
	if deployment == nil || deployment.Phase != deploymentErrorPhase {
		return nil
	}

	key := "digitalocean:" + deployment.ID
	subject := "[DigitalOcean] Deployment failed"
	body := fmt.Sprintf(
		"Deployment %s failed.\nCause: %s",
		deployment.ID,
		deployment.Cause,
	)
	return j.notifyOnce(ctx, key, subject, body)
}

// notifyOnce sends subject/body and records key as notified, unless key was
// already notified. The dedup key is only inserted after a successful send,
// so a failed send is retried on the next run instead of being silently
// dropped.
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

	if err = j.mail.Send(ctx, subject, body); errors.Is(err, mailer.ErrNotConfigured) {
		return nil
	} else if err != nil {
		return err
	}

	return j.notified.Insert(ctx, key)
}
