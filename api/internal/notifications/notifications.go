// Package notifications decouples callers from the latency of a
// mailer.Client send: every notification is deposited on a single background
// worker so a scheduled job's Run (see observability/jobs.WeeklyDigestJob)
// or an HTTP handler (see family.Service.InviteByEmail) never blocks on
// Resend's network round trip, and every enqueued notification is delivered
// strictly in enqueue order (issue #923).
//
// Delivery is email-only: EnqueueEmail sends to the fixed digest recipient,
// EnqueueTo to an arbitrary address. The email/Slack channel switch that
// once fanned alerts out per global.notification_channel_config was removed
// with its last producer when alerting moved wholesale to Grafana (issue
// #1530, superseding docs/adr-0020).
package notifications

import (
	"context"
	"log/slog"

	"tools.xdoubleu.com/internal/mailer"
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
// attempted. err mirrors what a single mailer.Client call returns: nil on
// success, mailer.ErrNotConfigured when the mailer is unconfigured, or the
// send/API error otherwise -- callers distinguish these the same way they
// always have. Returning a non-nil error from OnResult logs it via the
// underlying threading.WorkerPool.
type OnResult func(ctx context.Context, err error) error

// Service queues outgoing notifications and delivers them, in the order they
// were enqueued, on a single background worker.
type Service struct {
	pool *threading.WorkerPool
	mail mailer.Client
}

// New creates a Service. ctx governs the lifetime of the background worker,
// so it should be the application's own long-lived context, not a
// per-request one.
func New(ctx context.Context, logger *slog.Logger, mail mailer.Client) *Service {
	return &Service{
		pool: threading.NewWorkerPool(ctx, logger, singleWorker, queueSize),
		mail: mail,
	}
}

// EnqueueEmail queues subject/body for email delivery to the fixed
// recipient. Used by the weekly digests (docs/adr-0010).
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
