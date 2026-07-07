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
// It never writes to a book itself — that only happens when an admin resolves
// a proposal via BookService.ApplyResyncChoice.
//
// The job holds a reference to the progress WebSocket service so it can emit
// per-book progress events (X of N) over the /books/api/progress WebSocket.
type ResyncOpenLibraryJob struct {
	books *services.BookService
	ws    *progressws.Service

	armed   atomic.Bool
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

// Arm marks the job to scan the whole catalog on the next Run call.
func (j *ResyncOpenLibraryJob) Arm() {
	j.armed.Store(true)
}

func (j *ResyncOpenLibraryJob) Run(ctx context.Context, logger *slog.Logger) error {
	if !j.armed.Swap(false) {
		return nil
	}

	if !j.running.CompareAndSwap(false, true) {
		return nil
	}
	defer j.running.Store(false)

	var onProgress func(int, int)
	if j.ws != nil {
		id := j.ID()
		onProgress = func(processed, total int) {
			j.ws.UpdateProgress(id, processed, total)
		}
	}

	n, err := j.books.BuildResyncProposals(ctx, logger, onProgress)
	if n > 0 {
		logger.InfoContext(ctx, "flagged books with resync differences",
			slog.Int("count", n),
		)
	}
	return err
}
