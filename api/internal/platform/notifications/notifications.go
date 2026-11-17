// Package notifications delivers emails from a durable outbox so callers never
// block on Resend and a queued send survives a restart. Sends go out in enqueue
// order; a failed send retries with capped exponential backoff, then is marked
// failed with its error.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mailer"
)

const (
	// JobID identifies the outbox sweep in job stats.
	JobID = "notification_outbox"

	statusPending = "pending"

	sweepInterval = 5 * time.Second
	// jobRunEvery is how often the job queue sweeps, as a backstop to the
	// loop's own interval.
	jobRunEvery = time.Minute
	batchSize   = 50
	// lease hides a claimed row from other claimers while it is sent.
	lease = 2 * time.Minute

	defaultMaxAttempts = 5
	defaultBaseBackoff = 30 * time.Second
	maxBackoff         = time.Hour

	finishedRetention = 14 * 24 * time.Hour
	pruneEvery        = time.Hour
)

// OnResult runs in the sending process once a message is final, with the
// mailer.Client error (nil, mailer.ErrNotConfigured, or the last send error).
// It is lost if the process restarts before the send, so it must only add
// optional bookkeeping. A returned error is logged.
type OnResult func(ctx context.Context, err error) error

// Store persists outbox messages.
type Store interface {
	Enqueue(ctx context.Context, to, subject, body string) (int64, error)
	ClaimDue(
		ctx context.Context,
		now time.Time,
		lease time.Duration,
		limit int,
	) ([]models.OutboxMessage, error)
	MarkSent(ctx context.Context, id int64) error
	MarkRetry(ctx context.Context, id int64, next time.Time, lastErr string) error
	MarkFailed(ctx context.Context, id int64, lastErr string) error
	Depth(ctx context.Context) (models.OutboxDepth, error)
	PruneFinished(ctx context.Context, before time.Time) error
}

// Option tunes a Service.
type Option func(*Service)

// WithRetry sets the attempt cap (including the first send) and the backoff
// before the first retry, which doubles per attempt.
func WithRetry(maxAttempts int, baseBackoff time.Duration) Option {
	return func(s *Service) {
		s.maxAttempts = maxAttempts
		s.baseBackoff = baseBackoff
	}
}

// Service queues notifications and sends them from a background loop.
type Service struct {
	store       Store
	mail        mailer.Client
	logger      *slog.Logger
	ctx         context.Context
	maxAttempts int
	baseBackoff time.Duration
	wake        chan struct{}

	sweepMu  sync.Mutex // held for a whole sweep
	mu       sync.Mutex
	handlers map[int64]OnResult
	pruned   time.Time
}

// New creates a Service that keeps its queue in memory and sends each message
// once; ctx must be the app's long-lived context. Sends keep its values but
// not its cancellation, so Shutdown still drains what was queued.
func New(ctx context.Context, logger *slog.Logger, mail mailer.Client) *Service {
	return newService(ctx, logger, mail, newMemStore(), WithRetry(1, 0))
}

// NewDurable creates a Service over store: messages survive a restart and
// failed sends retry with backoff.
func NewDurable(
	ctx context.Context,
	logger *slog.Logger,
	mail mailer.Client,
	store Store,
	opts ...Option,
) *Service {
	return newService(ctx, logger, mail, store, opts...)
}

func newService(
	ctx context.Context,
	logger *slog.Logger,
	mail mailer.Client,
	store Store,
	opts ...Option,
) *Service {
	//nolint:exhaustruct // mutexes and pruned are zero values
	s := &Service{
		store:       store,
		mail:        mail,
		logger:      logger,
		ctx:         ctx,
		maxAttempts: defaultMaxAttempts,
		baseBackoff: defaultBaseBackoff,
		wake:        make(chan struct{}, 1),
		handlers:    make(map[int64]OnResult),
	}
	for _, opt := range opts {
		opt(s)
	}
	go s.loop()
	return s
}

// EnqueueEmail queues an email to the fixed recipient.
func (s *Service) EnqueueEmail(subject, body string, onResult OnResult) {
	s.enqueue("", subject, body, onResult)
}

// EnqueueTo queues subject/body for email delivery to an arbitrary recipient.
func (s *Service) EnqueueTo(to, subject, body string, onResult OnResult) {
	s.enqueue(to, subject, body, onResult)
}

func (s *Service) enqueue(to, subject, body string, onResult OnResult) {
	ctx := context.WithoutCancel(s.ctx)
	id, err := s.insert(ctx, to, subject, body, onResult)
	if err != nil {
		s.logger.ErrorContext(ctx, "notifications: enqueue failed",
			slog.Int64("id", id), slog.Any("error", err))
		if onResult != nil {
			s.report(ctx, onResult, err)
		}
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// insert stores m and registers onResult under the lock finish takes, so a
// sweep can't complete the message before its handler is known.
func (s *Service) insert(
	ctx context.Context,
	to, subject, body string,
	onResult OnResult,
) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := s.store.Enqueue(ctx, to, subject, body)
	if err == nil && onResult != nil {
		s.handlers[id] = onResult
	}
	return id, err
}

func (s *Service) loop() {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			// Drain what is due now; the rest stays queued for the next process.
			s.sweepLogged()
			return
		case <-s.wake:
		case <-ticker.C:
		}
		s.sweepLogged()
	}
}

func (s *Service) sweepLogged() {
	ctx := context.WithoutCancel(s.ctx)
	if err := s.sweep(ctx); err != nil {
		s.logger.WarnContext(ctx, "notifications: sweep", slog.Any("error", err))
	}
}

// ID, RunEvery and Run make the Service a threading.Scheduled job, so sweeps
// and their failures show in get_job_stats.
func (s *Service) ID() string { return JobID }

func (s *Service) RunEvery() time.Duration { return jobRunEvery }

// Run sends every due message and errors when a send failed, whether it will
// retry or was given up.
func (s *Service) Run(ctx context.Context, _ *slog.Logger) error {
	return s.sweep(ctx)
}

// Depth reports pending and failed outbox rows.
func (s *Service) Depth(ctx context.Context) (models.OutboxDepth, error) {
	return s.store.Depth(ctx)
}

func (s *Service) sweep(ctx context.Context) error {
	s.sweepMu.Lock()
	defer s.sweepMu.Unlock()

	var failures []error
	for {
		msgs, err := s.store.ClaimDue(ctx, time.Now(), lease, batchSize)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, m := range msgs {
			if sendErr := s.deliver(ctx, m); sendErr != nil {
				failures = append(failures, sendErr)
			}
		}
		if len(msgs) < batchSize {
			break
		}
	}
	s.prune(ctx)

	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%d notification sends failed: %w",
		len(failures), errors.Join(failures...))
}

// deliver sends m once and records the outcome; it returns the send error.
func (s *Service) deliver(ctx context.Context, m models.OutboxMessage) error {
	var err error
	if m.To == "" {
		err = s.mail.Send(ctx, m.Subject, m.Body)
	} else {
		err = s.mail.SendTo(ctx, m.To, m.Subject, m.Body)
	}

	switch {
	case err == nil || errors.Is(err, mailer.ErrNotConfigured):
		s.record(ctx, m.ID, s.store.MarkSent(ctx, m.ID))
		s.finish(ctx, m.ID, err)
		return nil // not configured is degraded, not failed
	case m.Attempts >= s.maxAttempts:
		s.record(ctx, m.ID, s.store.MarkFailed(ctx, m.ID, err.Error()))
		s.finish(ctx, m.ID, err)
	default:
		next := time.Now().Add(s.backoff(m.Attempts))
		s.record(ctx, m.ID, s.store.MarkRetry(ctx, m.ID, next, err.Error()))
	}
	return err
}

// backoff doubles per attempt, capped at maxBackoff.
func (s *Service) backoff(attempts int) time.Duration {
	d := s.baseBackoff
	for i := 1; i < attempts && d < maxBackoff; i++ {
		d *= 2
	}
	return min(d, maxBackoff)
}

func (s *Service) record(ctx context.Context, id int64, err error) {
	if err != nil {
		s.logger.ErrorContext(ctx, "notifications: outbox update failed",
			slog.Int64("id", id), slog.Any("error", err))
	}
}

// finish runs and forgets the message's OnResult, if this process holds it.
func (s *Service) finish(ctx context.Context, id int64, sendErr error) {
	s.mu.Lock()
	h := s.handlers[id]
	delete(s.handlers, id)
	s.mu.Unlock()
	if h != nil {
		s.report(ctx, h, sendErr)
	}
}

func (s *Service) report(ctx context.Context, h OnResult, sendErr error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.ErrorContext(ctx, "notifications: OnResult panicked",
				slog.Any("panic", r))
		}
	}()
	if err := h(ctx, sendErr); err != nil {
		s.logger.ErrorContext(ctx, "notifications: send failed", slog.Any("error", err))
	}
}

func (s *Service) prune(ctx context.Context) {
	if time.Since(s.pruned) < pruneEvery {
		return
	}
	s.pruned = time.Now()
	if err := s.store.PruneFinished(ctx, time.Now().Add(-finishedRetention)); err != nil {
		s.logger.ErrorContext(ctx, "notifications: prune failed", slog.Any("error", err))
	}
}

// WaitUntilDone sweeps until nothing is pending (tests only).
func (s *Service) WaitUntilDone() {
	for {
		s.sweepLogged() // don't wait out the sweep interval for retries
		s.sweepMu.Lock()
		d, err := s.store.Depth(context.WithoutCancel(s.ctx))
		s.sweepMu.Unlock()
		if err != nil || d.Pending == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// Shutdown sends what is due and blocks until in-flight sends finish or ctx is
// done. Messages still retrying stay queued for the next process.
func (s *Service) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.sweepLogged()
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
