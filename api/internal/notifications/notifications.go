// Package notifications decouples callers from the latency of a
// mailer.Client send: every email is deposited on a single background
// worker so a scheduled job's Run (see observability/jobs.IssueNotifierJob)
// or an HTTP handler (see family.Service.InviteByEmail) never blocks on
// Resend's network round trip, and every enqueued email is delivered
// strictly in enqueue order (issue #923).
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
// attempted. err is exactly what the underlying mailer.Client call
// returned: nil on success, mailer.ErrNotConfigured when the mailer isn't
// set up, or the send/API error otherwise -- callers distinguish these the
// same way they did calling mailer.Client directly. Returning a non-nil
// error from OnResult logs it via the underlying threading.WorkerPool.
type OnResult func(ctx context.Context, err error) error

// Service queues outgoing notification emails and delivers them, in the
// order they were enqueued, on a single background worker.
type Service struct {
	pool *threading.WorkerPool
	mail mailer.Client
}

// New creates a Service backed by mail. ctx governs the lifetime of the
// background worker, so it should be the application's own long-lived
// context, not a per-request one.
func New(ctx context.Context, logger *slog.Logger, mail mailer.Client) *Service {
	return &Service{
		pool: threading.NewWorkerPool(ctx, logger, singleWorker, queueSize),
		mail: mail,
	}
}

// Enqueue queues subject/body for delivery to the mailer's fixed recipient.
func (s *Service) Enqueue(subject, body string, onResult OnResult) {
	s.enqueue(onResult, func(ctx context.Context) error {
		return s.mail.Send(ctx, subject, body)
	})
}

// EnqueueTo queues subject/body for delivery to an arbitrary recipient.
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

// WaitUntilDone blocks until every queued email (and its OnResult callback)
// has finished processing. Intended for tests only.
func (s *Service) WaitUntilDone() {
	s.pool.WaitUntilDone()
}
