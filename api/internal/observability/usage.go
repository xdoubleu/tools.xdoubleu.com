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
	// usageRetention bounds global.usage_daily (~13 months so year-over-year
	// comparisons stay possible).
	usageRetention = 400 * 24 * time.Hour
	pruneInterval  = 24 * time.Hour
)

type usageKey struct {
	day      string
	app      string
	endpoint string
}

// usageStore is the slice of UsageRepository UsageRecorder needs.
type usageStore interface {
	Flush(ctx context.Context, entries []models.UsageEntry) error
	PruneOlderThan(ctx context.Context, cutoff time.Time) error
}

// usageCounter accumulates one (day, app, endpoint) bucket.
type usageCounter struct {
	count int64
	bytes int64
}

// UsageRecorder counts requests and response bytes per (day, app, endpoint)
// in memory and periodically flushes them into global.usage_daily. Losing at
// most one flush interval of counts on shutdown is an accepted trade-off for
// keeping request handling free of DB writes.
type UsageRecorder struct {
	logger    *slog.Logger
	repo      usageStore
	mu        sync.Mutex
	counts    map[usageKey]usageCounter
	lastPrune time.Time
}

func NewUsageRecorder(logger *slog.Logger, db postgres.DB) *UsageRecorder {
	//nolint:exhaustruct //mu and lastPrune start zero-valued on purpose
	return &UsageRecorder{
		logger: logger,
		repo:   repositories.NewUsageRepository(db),
		counts: make(map[usageKey]usageCounter),
	}
}

// Record counts one request and the response bytes it served. Safe for
// concurrent use.
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

// Start launches the flush loop. It runs for the lifetime of ctx.
func (u *UsageRecorder) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				u.flushTick(ctx)
				return
			case <-ticker.C:
				u.flushTick(ctx)
			}
		}
	}()
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

// Flush writes the accumulated counts to the database and clears them.
// On error the batch is merged back so the counts survive for a retry.
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
