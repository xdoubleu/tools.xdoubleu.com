// Package notifications decouples callers from the latency of a
// mailer.Client send: every notification is deposited on a single background
// worker so a scheduled job's Run (see observability/jobs.WeeklyDigestJob)
// or an HTTP handler (see family.Service.InviteByEmail) never blocks on
// Resend's network round trip, and every enqueued notification is delivered
// strictly in enqueue order (issue #923).
//
// Alert notifications enqueued via Enqueue fan out to email and/or Slack
// according to the global channel switch in
// global.notification_channel_config (issue #1482). EnqueueEmail and
// EnqueueTo always use email regardless of that switch — the weekly digests
// and family invites are email-only by design (docs/adr-0020).
package notifications

import (
	"context"
	"errors"
	"log/slog"

	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/slack"
	"tools.xdoubleu.com/internal/threading"
)

// singleWorker keeps delivery strictly FIFO -- more than one worker would
// let two sends race and land out of enqueue order.
const singleWorker = 1

// queueSize is generous relative to how rarely these notifications fire
// (a 5-minute poll job, occasional family invites); it exists only so a
// burst never blocks the enqueueing caller.
const queueSize = 64

// OnResult is invoked, on the delivery worker, once a send has been
// attempted. err mirrors what a single mailer.Client call used to return:
// nil on success, mailer.ErrNotConfigured when no configured channel could
// deliver, or the send/API error otherwise -- callers distinguish these the
// same way they always have. Returning a non-nil error from OnResult logs it
// via the underlying threading.WorkerPool.
type OnResult func(ctx context.Context, err error) error

// channelConfigGetter reads the global email/Slack delivery switch. Backed
// by *repositories.NotificationChannelConfigRepository in production.
type channelConfigGetter interface {
	Get(ctx context.Context) (repositories.NotificationChannelConfig, error)
}

// Service queues outgoing notifications and delivers them, in the order they
// were enqueued, on a single background worker.
type Service struct {
	pool       *threading.WorkerPool
	mail       mailer.Client
	slack      slack.Client
	channelCfg channelConfigGetter
}

// New creates a Service. ctx governs the lifetime of the background worker,
// so it should be the application's own long-lived context, not a
// per-request one.
func New(
	ctx context.Context,
	logger *slog.Logger,
	mail mailer.Client,
	slackClient slack.Client,
	channelCfg channelConfigGetter,
) *Service {
	return &Service{
		pool:       threading.NewWorkerPool(ctx, logger, singleWorker, queueSize),
		mail:       mail,
		slack:      slackClient,
		channelCfg: channelCfg,
	}
}

// NewEmailOnly creates a Service with no Slack channel wired: every Enqueue
// is delivered over email, exactly as before issue #1482. For callers that
// never fan out to Slack (tests, and any future email-only context).
func NewEmailOnly(
	ctx context.Context, logger *slog.Logger, mail mailer.Client,
) *Service {
	return New(ctx, logger, mail, unconfiguredSlack{}, emailOnlyConfig{})
}

type unconfiguredSlack struct{}

func (unconfiguredSlack) Send(_ context.Context, _, _, _ string) error {
	return slack.ErrNotConfigured
}

type emailOnlyConfig struct{}

func (emailOnlyConfig) Get(
	_ context.Context,
) (repositories.NotificationChannelConfig, error) {
	return repositories.NotificationChannelConfig{
		ChannelMode:     repositories.ChannelModeEmail,
		SlackWebhookURL: "",
	}, nil
}

// Enqueue queues an alert for delivery to whichever channel(s) the global
// switch selects (email, Slack, or both).
func (s *Service) Enqueue(subject, body string, onResult OnResult) {
	s.enqueue(onResult, func(ctx context.Context) error {
		return s.deliverAlert(ctx, subject, body)
	})
}

// EnqueueEmail queues subject/body for email delivery to the fixed
// recipient, bypassing the channel switch. Used by the weekly digests, which
// are email-only regardless of the switch (docs/adr-0020).
func (s *Service) EnqueueEmail(subject, body string, onResult OnResult) {
	s.enqueue(onResult, func(ctx context.Context) error {
		return s.mail.Send(ctx, subject, body)
	})
}

// EnqueueTo queues subject/body for email delivery to an arbitrary recipient.
func (s *Service) EnqueueTo(to, subject, body string, onResult OnResult) {
	s.enqueue(onResult, func(ctx context.Context) error {
		return s.mail.SendTo(ctx, to, subject, body)
	})
}

// deliverAlert sends one alert over the configured channel(s). When the mode
// is Slack-only and Slack does not succeed (a send error, or no webhook URL
// configured at all), it also falls back to email so choosing "slack" can
// never silently drop an alert.
func (s *Service) deliverAlert(ctx context.Context, subject, body string) error {
	mode := repositories.ChannelModeEmail
	webhookURL := ""
	if cfg, err := s.channelCfg.Get(ctx); err == nil {
		if repositories.IsValidChannelMode(cfg.ChannelMode) {
			mode = cfg.ChannelMode
		}
		webhookURL = cfg.SlackWebhookURL
	}

	var results []error
	switch mode {
	case repositories.ChannelModeEmail:
		results = append(results, s.mail.Send(ctx, subject, body))
	case repositories.ChannelModeBoth:
		results = append(results, s.mail.Send(ctx, subject, body))
		results = append(results, s.slack.Send(ctx, webhookURL, subject, body))
	case repositories.ChannelModeSlack:
		slackErr := s.slack.Send(ctx, webhookURL, subject, body)
		results = append(results, slackErr)
		if slackErr != nil {
			results = append(results, s.mail.Send(ctx, subject, body))
		}
	}

	return combineResults(results)
}

// combineResults folds the per-channel outcomes into the single error
// contract OnResult callbacks expect: nil if any channel delivered,
// mailer.ErrNotConfigured if every attempted channel was simply unconfigured
// (a benign degraded state — nothing sent, nothing retried differently), and
// the first real send error otherwise (so the caller does not persist its
// dedup key and retries next run).
func combineResults(results []error) error {
	delivered := false
	var realErr error
	for _, err := range results {
		switch {
		case err == nil:
			delivered = true
		case errors.Is(err, mailer.ErrNotConfigured),
			errors.Is(err, slack.ErrNotConfigured):
			// unconfigured channel — degraded, not failed
		default:
			if realErr == nil {
				realErr = err
			}
		}
	}

	if delivered {
		return nil
	}
	if realErr != nil {
		return realErr
	}
	return mailer.ErrNotConfigured
}

func (s *Service) enqueue(onResult OnResult, send func(ctx context.Context) error) {
	s.pool.EnqueueWork(func(ctx context.Context, _ *slog.Logger) error {
		err := send(ctx)
		if onResult == nil {
			return err
		}
		return onResult(ctx, err)
	})
}

// WaitUntilDone blocks until every queued notification (and its OnResult
// callback) has finished processing. Intended for tests only.
func (s *Service) WaitUntilDone() {
	s.pool.WaitUntilDone()
}
