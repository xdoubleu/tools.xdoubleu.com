package jobs

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/internal/progressws"
)

// ResyncOpenLibraryJob scans the whole catalog for metadata differences
// against Open Library, Google Books, and UniCat, and stores what it finds
// for the admin resync wizard to review. It is on-demand only: it must be
// armed via Arm() before Run() does any work, so the unavoidable startup run
// (added by JobQueue.AddJob) and the daily scheduler tick are no-ops.
//
// force bypasses the skip-if-known cache for every source, not just Google
// Books — see BookService.BuildResyncProposals.
//
// It never writes to a book itself — that only happens when an admin resolves
// a proposal via BookService.ApplyResyncChoice.
//
// The job holds a reference to the progress WebSocket service so it can emit
// per-book progress events (X of N) over the /books/api/progress WebSocket.
type ResyncOpenLibraryJob struct {
	books *services.BookService
	ws    *progressws.Service

	armed   atomic.Bool
	force   atomic.Bool
	running atomic.Bool
}

func NewResyncOpenLibraryJob(
	books *services.BookService,
	ws *progressws.Service,
) *ResyncOpenLibraryJob {
	//nolint:exhaustruct //armed + running are atomic.Bool; zero value = false
	return &ResyncOpenLibraryJob{books: books, ws: ws}
}

func (j *ResyncOpenLibraryJob) ID() string {
	return "resync-openlibrary"
}

func (j *ResyncOpenLibraryJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

// Arm marks the job to scan the whole catalog on the next Run call. force
// bypasses every source's skip-if-known cache for that run — see
// BookService.BuildResyncProposals.
func (j *ResyncOpenLibraryJob) Arm(force bool) {
	j.armed.Store(true)
	j.force.Store(force)
}

func (j *ResyncOpenLibraryJob) Run(ctx context.Context, logger *slog.Logger) error {
	if !j.armed.Swap(false) {
		return nil
	}

	if !j.running.CompareAndSwap(false, true) {
		return nil
	}
	defer j.running.Store(false)

	force := j.force.Swap(false)

	var onProgress func(int, int, bool)
	if j.ws != nil {
		id := j.ID()
		onProgress = func(processed, total int, quotaReached bool) {
			j.ws.UpdateProgress(id, processed, total, quotaReached)
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
