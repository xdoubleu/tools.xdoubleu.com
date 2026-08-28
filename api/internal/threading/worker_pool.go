package threading

import (
	"context"
	"log/slog"
	"sync"

	"tools.xdoubleu.com/internal/sentrytools"
)

// DoWork describes the interface for work executed by the workers.
type DoWork = func(ctx context.Context, logger *slog.Logger) error

// WorkerPool is used to divide [Subscriber]s between [Worker]s.
// This prevents one [Worker] of being very busy.
type WorkerPool struct {
	ctx     context.Context
	logger  *slog.Logger
	workers []Worker
	queue   chan DoWork
	// wg tracks outstanding work so WaitUntilDone can block deterministically
	// -- IsWorkRemaining (len(queue)>0 || IsDoingWork()) has a window right
	// after a worker dequeues but before it flips isDoingWork where both are
	// false even though the work hasn't run yet, letting WaitUntilDone
	// return early (observed as an intermittent WaitUntilDone-then-assert
	// failure in notifications tests). A pointer, not a value, because
	// EventQueue embeds a WorkerPool by value (workerPool WorkerPool) while
	// each Worker's goroutine holds a *WorkerPool back to the original —
	// a value sync.WaitGroup would silently split into two independent
	// counters across that copy the same way a value chan/slice wouldn't.
	wg *sync.WaitGroup
}

// NewWorkerPool creates a new [WorkerPool].
func NewWorkerPool(
	ctx context.Context,
	logger *slog.Logger,
	amountWorkers int,
	queueSize int,
) *WorkerPool {
	pool := &WorkerPool{
		ctx:     ctx,
		logger:  logger,
		workers: make([]Worker, amountWorkers),
		queue:   make(chan DoWork, queueSize),
		wg:      &sync.WaitGroup{},
	}

	pool.createWorkers(amountWorkers)
	pool.Start()

	return pool
}

// Active checks if the [WorkerPool] is active
// by checking if any [Worker] is active.
func (pool *WorkerPool) Active() bool {
	for i := range pool.workers {
		if pool.workers[i].Active() {
			return true
		}
	}
	return false
}

// IsDoingWork checks if the [WorkerPool] is still processing work.
func (pool *WorkerPool) IsDoingWork() bool {
	for i := range pool.workers {
		if pool.workers[i].IsDoingWork() {
			return true
		}
	}
	return false
}

// Start starts [Worker]s of a [WorkerPool] if they weren't active yet.
func (pool *WorkerPool) Start() {
	for i := range pool.workers {
		go sentrytools.SetupGoRoutineHub(
			pool.ctx,
			pool.logger,
			pool.workers[i].Run,
		)
	}
}

// EnqueueWork puts an work on the queue.
func (pool *WorkerPool) EnqueueWork(doWork DoWork) {
	pool.wg.Add(1)
	pool.queue <- doWork
}

// IsWorkRemaining checks if there is still work on the queue.
func (pool *WorkerPool) IsWorkRemaining() bool {
	return len(pool.queue) > 0 || pool.IsDoingWork()
}

// WaitUntilDone blocks until every enqueued work item has finished running.
func (pool *WorkerPool) WaitUntilDone() {
	pool.wg.Wait()
}

// Stop stops all workers.
func (pool *WorkerPool) Stop() {
	for i := range pool.workers {
		pool.workers[i].Stop()
	}
}

func (pool *WorkerPool) createWorkers(amountWorkers int) {
	for i := 0; i < amountWorkers; i++ {
		pool.workers[i] = NewWorker(i, pool)
	}
}
