package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

const (
	dayFormat = "2006-01-02"
	// usageRetention (~13 months) keeps year-over-year comparisons possible.
	usageRetention = 400 * 24 * time.Hour
	pruneInterval  = 24 * time.Hour
)

type usageKey struct {
	day      string
	app      string
	endpoint string
}

type usageStore interface {
	Flush(ctx context.Context, entries []models.UsageEntry) error
	PruneOlderThan(ctx context.Context, cutoff time.Time) error
}

type usageCounter struct {
	count int64
	bytes int64
}

// UsageRecorder counts requests and bytes per (day, app, endpoint) in memory
// and flushes to global.usage_daily; Stop flushes what's left.
type UsageRecorder struct {
	logger    *slog.Logger
	repo      usageStore
	mu        sync.Mutex
	counts    map[usageKey]usageCounter
	lastPrune time.Time
	stop      chan struct{} // nil until Start
	stopped   chan struct{}
	stopOnce  sync.Once
}

func NewUsageRecorder(logger *slog.Logger, db postgres.DB) *UsageRecorder {
	//nolint:exhaustruct //mu, lastPrune and the Start/Stop fields start zero-valued
	return &UsageRecorder{
		logger: logger,
		repo:   repositories.NewUsageRepository(db),
		counts: make(map[usageKey]usageCounter),
	}
}

// Record counts one request. Safe for concurrent use.
func (u *UsageRecorder) Record(app, endpoint string, bytes int64) {
	key := usageKey{
		day:      time.Now().UTC().Format(dayFormat),
		app:      app,
		endpoint: endpoint,
	}

	u.mu.Lock()
	entry := u.counts[key]
	entry.count++
	entry.bytes += bytes
	u.counts[key] = entry
	u.mu.Unlock()
}

// Start flushes every interval until Stop. Flushes keep ctx's values but not
// its cancellation, so they still work while the server drains.
func (u *UsageRecorder) Start(ctx context.Context, interval time.Duration) {
	ctx = context.WithoutCancel(ctx)
	u.stop = make(chan struct{})
	u.stopped = make(chan struct{})

	go func() {
		defer close(u.stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-u.stop:
				return
			case <-ticker.C:
				u.flushTick(ctx)
			}
		}
	}()
}

// Stop ends Start's loop, then flushes the remaining counts with ctx. Without
// Start it does nothing.
func (u *UsageRecorder) Stop(ctx context.Context) error {
	if u.stop == nil {
		return nil
	}
	u.stopOnce.Do(func() { close(u.stop) })

	select {
	case <-u.stopped:
	case <-ctx.Done():
		return ctx.Err()
	}
	return u.Flush(ctx)
}

// flushTick recovers panics so one bad flush cannot kill the loop.
func (u *UsageRecorder) flushTick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			u.logger.ErrorContext(
				ctx,
				"usage flush panicked",
				slog.Any("panic", r),
			)
		}
	}()

	if err := u.Flush(ctx); err != nil {
		u.logger.ErrorContext(
			ctx,
			"failed to flush usage counts",
			essentialogger.ErrAttr(err),
		)
	}
}

// Flush writes and clears the counts; on error they are merged back.
func (u *UsageRecorder) Flush(ctx context.Context) error {
	u.mu.Lock()
	batch := u.counts
	u.counts = make(map[usageKey]usageCounter)
	u.mu.Unlock()

	if len(batch) > 0 {
		entries := make([]models.UsageEntry, 0, len(batch))
		for key, counter := range batch {
			day, err := time.Parse(dayFormat, key.day)
			if err != nil {
				continue
			}
			entries = append(entries, models.UsageEntry{
				Day:      day,
				App:      key.app,
				Endpoint: key.endpoint,
				Count:    counter.count,
				Bytes:    counter.bytes,
			})
		}

		if err := u.repo.Flush(ctx, entries); err != nil {
			u.mu.Lock()
			for key, counter := range batch {
				merged := u.counts[key]
				merged.count += counter.count
				merged.bytes += counter.bytes
				u.counts[key] = merged
			}
			u.mu.Unlock()
			return err
		}
	}

	return u.maybePrune(ctx)
}

func (u *UsageRecorder) maybePrune(ctx context.Context) error {
	u.mu.Lock()
	due := time.Since(u.lastPrune) >= pruneInterval
	if due {
		u.lastPrune = time.Now()
	}
	u.mu.Unlock()

	if !due {
		return nil
	}
	return u.repo.PruneOlderThan(ctx, time.Now().Add(-usageRetention))
}
