package jobs

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"tools.xdoubleu.com/apps/reading/internal/services"
	"tools.xdoubleu.com/internal/progressws"
)

// ResyncMetadataJob scans the whole catalog for metadata differences
// against UniCat and Hardcover, and stores what it finds for
// the admin resync wizard to review. It is on-demand only: it must be armed
// via Arm() before Run() does any work, so the unavoidable startup run
// (added by JobQueue.AddJob) and the daily scheduler tick are no-ops.
//
// force bypasses the skip-if-known cache for every source — see
// BookService.BuildResyncProposals.
//
// It never writes to a book itself — that only happens when an admin resolves
// a proposal via BookService.ApplyResyncChoice.
//
// The job holds a reference to the progress WebSocket service so it can emit
// per-book progress events (X of N) over the /books/api/progress WebSocket.
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

func (j *ResyncMetadataJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

// Arm marks the job to scan the whole catalog on the next Run call. force
// bypasses every source's skip-if-known cache for that run — see
// BookService.BuildResyncProposals.
func (j *ResyncMetadataJob) Arm(force bool) {
	j.armed.Store(true)
	j.force.Store(force)
}

// Cancel stops an in-progress scan, if one is running. A no-op otherwise —
// there's nothing to stop between runs.
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
