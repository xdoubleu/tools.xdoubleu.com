package jobs

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"tools.xdoubleu.com/apps/backlog/internal/services"
)

// ResyncOpenLibraryJob re-fetches Open Library metadata and clears cached
// covers for every book in the catalog that has an ISBN13. It is on-demand
// only: it must be armed via Arm() before Run() does any work, so the
// unavoidable startup run (added by JobQueue.AddJob) and the daily scheduler
// tick are no-ops.
type ResyncOpenLibraryJob struct {
	books   *services.BookService
	armed   atomic.Bool
	running atomic.Bool
}

func NewResyncOpenLibraryJob(books *services.BookService) *ResyncOpenLibraryJob {
	//nolint:exhaustruct //armed + running are atomic.Bool; zero value = false
	return &ResyncOpenLibraryJob{books: books}
}

func (j *ResyncOpenLibraryJob) ID() string {
	return "resync-openlibrary"
}

func (j *ResyncOpenLibraryJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

// Arm marks the job to actually do work on the next Run call.
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

	n, err := j.books.ResyncAllFromOpenLibrary(ctx, logger)
	if n > 0 {
		logger.InfoContext(ctx, "resynced books from open library",
			slog.Int("count", n),
		)
	}
	return err
}
