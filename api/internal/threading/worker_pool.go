package threading

import (
	"context"
	"log/slog"
	"sync"

	"tools.xdoubleu.com/internal/sentrytools"
)

// DoWork describes the interface for work executed by the workers.
type DoWork = func(ctx context.Context, logger *slog.Logger) error

// WorkerPool runs queued work on a set of [Worker]s.
type WorkerPool struct {
	ctx     context.Context
	logger  *slog.Logger
	workers []Worker
	queue   chan DoWork
	// wg makes WaitUntilDone deterministic: IsWorkRemaining has a gap between
	// dequeue and isDoingWork. A pointer because EventQueue copies WorkerPool by
	// value while workers hold the original.
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

// Active reports whether any [Worker] is active.
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

// EnqueueWork puts work on the queue.
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
