// Package threading provides tools for multi threading applications.
package threading

import (
	"context"
	"log/slog"
	"sync"
)

// Worker is used to handle work of the [WorkerPool].
type Worker struct {
	id            int
	active        bool
	activeMu      *sync.RWMutex
	isDoingWork   bool
	isDoingWorkMu *sync.RWMutex
	pool          *WorkerPool
}

// NewWorker creates a new [Worker].
func NewWorker(id int, pool *WorkerPool) Worker {
	worker := Worker{
		id:            id,
		active:        false,
		activeMu:      &sync.RWMutex{},
		isDoingWork:   false,
		isDoingWorkMu: &sync.RWMutex{},
		pool:          pool,
	}
	return worker
}

// Active fetches the current state of the worker.
func (worker *Worker) Active() bool {
	worker.activeMu.RLock()
	defer worker.activeMu.RUnlock()

	return worker.active
}

// IsDoingWork fetches the current state of the worker.
func (worker *Worker) IsDoingWork() bool {
	worker.isDoingWorkMu.RLock()
	defer worker.isDoingWorkMu.RUnlock()

	return worker.isDoingWork
}

// Stop stops the worker.
func (worker *Worker) Stop() {
	worker.activeMu.Lock()
	defer worker.activeMu.Unlock()

	worker.active = false
}

// Run makes [Worker] start doing work.
func (worker *Worker) Run(ctx context.Context, logger *slog.Logger) error {
	worker.activeMu.Lock()
	worker.active = true
	worker.activeMu.Unlock()

	for worker.Active() {
		worker.do(ctx, logger, <-worker.pool.queue)
	}

	return nil
}

// do runs one item; a panic is logged at Error so the worker survives it.
func (worker *Worker) do(ctx context.Context, logger *slog.Logger, doWork DoWork) {
	worker.isDoingWorkMu.Lock()
	worker.isDoingWork = true
	worker.isDoingWorkMu.Unlock()

	defer func() {
		worker.isDoingWorkMu.Lock()
		worker.isDoingWork = false
		worker.isDoingWorkMu.Unlock()
		worker.pool.inflight.done()
	}()
	defer Recover(ctx, logger, "worker")

	if err := doWork(ctx, logger); err != nil {
		logger.ErrorContext(ctx, err.Error())
	}
}
