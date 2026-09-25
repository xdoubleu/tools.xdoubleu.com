package jobs

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/internal/progressws"
)

// ResyncMetadataJob scans the catalog against UniCat and Hardcover and stores
// proposals for the admin wizard, reporting progress over the progress
// WebSocket. It is trigger-only (no RunEvery): StartResync Arms it and
// force-runs it; the armed guard is a second check. It never writes to books.
type ResyncMetadataJob struct {
	books *services.BookService
	ws    *progressws.Service

	armed   atomic.Bool
	force   atomic.Bool
	running atomic.Bool

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewResyncMetadataJob(
	books *services.BookService,
	ws *progressws.Service,
) *ResyncMetadataJob {
	//nolint:exhaustruct //armed + running are atomic.Bool; zero value = false
	return &ResyncMetadataJob{books: books, ws: ws}
}

func (j *ResyncMetadataJob) ID() string {
	return "resync-books"
}

// Arm makes the next Run scan the whole catalog; force bypasses the
// skip-if-known cache.
func (j *ResyncMetadataJob) Arm(force bool) {
	j.armed.Store(true)
	j.force.Store(force)
}

// Cancel stops an in-progress scan, if any.
func (j *ResyncMetadataJob) Cancel() {
	j.mu.Lock()
	cancel := j.cancel
	j.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (j *ResyncMetadataJob) Run(ctx context.Context, logger *slog.Logger) error {
	if !j.armed.Swap(false) {
		return nil
	}

	if !j.running.CompareAndSwap(false, true) {
		return nil
	}
	defer j.running.Store(false)

	ctx, cancel := context.WithCancel(ctx)
	j.mu.Lock()
	j.cancel = cancel
	j.mu.Unlock()
	defer func() {
		j.mu.Lock()
		j.cancel = nil
		j.mu.Unlock()
		cancel()
	}()

	force := j.force.Swap(false)

	var onProgress func(int, int)
	if j.ws != nil {
		id := j.ID()
		onProgress = func(processed, total int) {
			j.ws.UpdateProgress(id, processed, total)
		}
	}

	n, err := j.books.BuildResyncProposals(ctx, logger, onProgress, force)
	if n > 0 {
		logger.InfoContext(ctx, "flagged books with resync differences",
			slog.Int("count", n),
		)
	}
	return err
}
