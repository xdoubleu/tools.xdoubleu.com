package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/backlog/internal/services"
)

// RelocateFilesJob is a one-shot job that migrates any book_files rows still
// using the legacy flat storage scheme (books/<checksum><ext>) to the new
// per-book folder scheme (books/<bookID>/<checksum><ext>). It is idempotent:
// rows that already use the per-book scheme are skipped. Once all rows are
// migrated the job becomes a no-op on subsequent runs.
type RelocateFilesJob struct {
	bookService *services.BookService
}

func NewRelocateFilesJob(bookService *services.BookService) RelocateFilesJob {
	return RelocateFilesJob{bookService: bookService}
}

func (j RelocateFilesJob) ID() string {
	return "relocate-files"
}

// RunEvery runs once per day. Since it is idempotent and becomes a no-op once
// all rows are migrated, the schedule does not matter after the first run.
func (j RelocateFilesJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

func (j RelocateFilesJob) Run(ctx context.Context, logger *slog.Logger) error {
	n, err := j.bookService.RelocateFlatKeyFiles(ctx, logger)
	if err != nil {
		return fmt.Errorf("relocate flat-key files: %w", err)
	}

	if n > 0 {
		logger.InfoContext(ctx, "relocated book files to per-book folders",
			slog.Int("count", n),
		)
	}

	return nil
}
