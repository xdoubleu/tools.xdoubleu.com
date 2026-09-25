// Package notifications delivers emails on a single background worker so
// callers never block on Resend, strictly in enqueue order.
package notifications

import (
	"context"
	"log/slog"

	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/threading"
)

// singleWorker keeps delivery FIFO.
const singleWorker = 1

// queueSize only needs to absorb bursts so enqueueing never blocks.
const queueSize = 64

// OnResult runs on the worker after a send with the mailer.Client error (nil,
// mailer.ErrNotConfigured, or the send error). A returned error is logged.
type OnResult func(ctx context.Context, err error) error

// Service delivers queued notifications in order on one worker.
type Service struct {
	pool *threading.WorkerPool
	mail mailer.Client
}

// New creates a Service; ctx must be the app's long-lived context.
func New(ctx context.Context, logger *slog.Logger, mail mailer.Client) *Service {
	return &Service{
		pool: threading.NewWorkerPool(ctx, logger, singleWorker, queueSize),
		mail: mail,
	}
}

// EnqueueEmail queues an email to the fixed recipient.
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

// WaitUntilDone blocks until the queue is drained (tests only).
func (s *Service) WaitUntilDone() {
	s.pool.WaitUntilDone()
}
