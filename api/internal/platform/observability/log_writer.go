package observability

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/models"
)

const (
	dropReasonQueueFull    = "queue_full"
	dropReasonInsertFailed = "insert_failed"
)

//nolint:gochecknoglobals //promauto registers on the default registry once
var logEntriesDropped = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "log_entries_dropped_total",
	Help: "API log records not written to global.log_entries, by reason.",
}, []string{"reason"})

type logBatchInserter interface {
	InsertBatch(ctx context.Context, entries []models.LogEntry) error
}

type logWriterOptions struct {
	queueSize     int
	batchSize     int
	flushInterval time.Duration
	insertTimeout time.Duration
}

// Production peaks near 13 inserts/s, so a 1024-entry queue rides out a
// stalled database for over a minute.
const (
	logQueueSize     = 1024
	logBatchSize     = 100
	logFlushInterval = time.Second
	logInsertTimeout = 2 * time.Second
)

func defaultLogWriterOptions() logWriterOptions {
	return logWriterOptions{
		queueSize:     logQueueSize,
		batchSize:     logBatchSize,
		flushInterval: logFlushInterval,
		insertTimeout: logInsertTimeout,
	}
}

// logWriter inserts queued entries in batches from one goroutine, so a slow
// database never blocks a log call. It never logs: its own failures would
// feed back into the queue.
type logWriter struct {
	inserter  logBatchInserter
	opts      logWriterOptions
	entries   chan models.LogEntry
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	// ctx parents every insert; it is tied to no request, and Close cancels it
	// once its own deadline passes.
	ctx    context.Context
	cancel context.CancelFunc
}

func newLogWriter(inserter logBatchInserter, opts logWriterOptions) *logWriter {
	ctx, cancel := context.WithCancel(context.Background())
	w := &logWriter{
		inserter:  inserter,
		opts:      opts,
		entries:   make(chan models.LogEntry, opts.queueSize),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		closeOnce: sync.Once{},
		ctx:       ctx,
		cancel:    cancel,
	}
	go w.run()
	return w
}

// enqueue never blocks: a full queue drops the entry.
func (w *logWriter) enqueue(entry models.LogEntry) {
	select {
	case <-w.stop:
		return
	default:
	}
	select {
	case w.entries <- entry:
	default:
		logEntriesDropped.WithLabelValues(dropReasonQueueFull).Inc()
	}
}

func (w *logWriter) run() {
	defer close(w.done)
	defer w.cancel()

	ticker := time.NewTicker(w.opts.flushInterval)
	defer ticker.Stop()

	batch := make([]models.LogEntry, 0, w.opts.batchSize)
	add := func(entry models.LogEntry) {
		batch = append(batch, entry)
		if len(batch) >= w.opts.batchSize {
			batch = w.flush(batch)
		}
	}
	for {
		select {
		case entry := <-w.entries:
			add(entry)
		case <-ticker.C:
			batch = w.flush(batch)
		case <-w.stop:
			for {
				select {
				case entry := <-w.entries:
					add(entry)
				default:
					w.flush(batch)
					return
				}
			}
		}
	}
}

// flush inserts batch and returns it emptied for reuse. A batch is all or
// none, so when the server rejects it (one bad row, e.g. a NUL byte) the rows
// are retried one by one; any other failure drops the batch.
func (w *logWriter) flush(batch []models.LogEntry) []models.LogEntry {
	if len(batch) == 0 {
		return batch
	}
	ctx, cancel := context.WithTimeout(w.ctx, w.opts.insertTimeout)
	defer cancel()

	failed := 0
	err := w.inserter.InsertBatch(ctx, batch)
	var pgErr *pgconn.PgError
	switch {
	case err == nil:
	case len(batch) > 1 && errors.As(err, &pgErr):
		for i := range batch {
			if w.inserter.InsertBatch(ctx, batch[i:i+1]) != nil {
				failed++
			}
		}
	default:
		failed = len(batch)
	}
	if failed > 0 {
		logEntriesDropped.WithLabelValues(dropReasonInsertFailed).
			Add(float64(failed))
	}
	return batch[:0]
}

// Close stops queueing and writes what is queued, giving up when ctx ends.
// Entries logged after Close still reach the wrapped handler, not the DB.
func (w *logWriter) Close(ctx context.Context) error {
	w.closeOnce.Do(func() { close(w.stop) })
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		w.cancel()
		return ctx.Err()
	}
}
