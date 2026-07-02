package jobs

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/internal/progressws"
)

// ResyncOpenLibraryJob re-fetches Open Library metadata and clears cached
// covers for books in the catalog that are missing metadata. It is on-demand
// only: it must be armed via Arm() or ArmFor() before Run() does any work, so
// the unavoidable startup run (added by JobQueue.AddJob) and the daily
// scheduler tick are no-ops.
//
// When armed via ArmFor(ids, force), only the given book IDs are processed and
// force-mode overwrites existing metadata. When armed via Arm() (or
// ArmFor(nil, false)), all books missing metadata are processed additively.
//
// The job holds a reference to the progress WebSocket service so it can emit
// per-book progress events (X of N) over the /backlog/api/progress WebSocket.
type ResyncOpenLibraryJob struct {
	books *services.BookService
	ws    *progressws.Service

	mu           sync.Mutex
	pendingIDs   []uuid.UUID
	pendingForce bool

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

// Arm marks the job to resync all books missing metadata on the next Run call.
// Equivalent to ArmFor(nil, false).
func (j *ResyncOpenLibraryJob) Arm() {
	j.ArmFor(nil, false)
}

// ArmFor marks the job to resync only the given book IDs on the next Run call.
// When force is true, existing metadata is overwritten with whatever the
// providers return. Pass nil ids to resync all books missing metadata.
func (j *ResyncOpenLibraryJob) ArmFor(ids []uuid.UUID, force bool) {
	j.mu.Lock()
	j.pendingIDs = ids
	j.pendingForce = force
	j.mu.Unlock()
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

	j.mu.Lock()
	ids := j.pendingIDs
	force := j.pendingForce
	j.pendingIDs = nil
	j.pendingForce = false
	j.mu.Unlock()

	var onProgress func(int, int)
	if j.ws != nil {
		id := j.ID()
		onProgress = func(processed, total int) {
			j.ws.UpdateProgress(id, processed, total)
		}
	}

	var (
		n   int
		err error
	)

	if len(ids) == 0 {
		n, err = j.books.ResyncAllFromOpenLibrary(ctx, logger, onProgress)
	} else {
		n, err = j.books.ResyncBooks(ctx, logger, ids, force, onProgress)
	}

	if n > 0 {
		logger.InfoContext(ctx, "resynced books from open library",
			slog.Int("count", n),
			slog.Bool("force", force),
		)
	}
	return err
}
