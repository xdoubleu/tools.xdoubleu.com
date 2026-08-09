// Package jobqueue schedules recurring background jobs. Unlike the
// now-removed essentia threading.JobQueue this used to sit on top of --
// whose AddJob always ran a job immediately and whose due-checking lived
// only in memory, both reset on every process restart -- it checks
// global.job_runs (the same table observability.TrackedJob already writes
// run history to) as the durable record of when a job last succeeded, so a
// restart doesn't re-run everything regardless of RunEvery. Execution still
// goes through threading.WorkerPool (issue #912), which has no such bug.
package jobqueue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/threading"
)

// tickInterval is how often the scheduler checks whether a job is due.
// ponytail: fixed floor instead of replicating essentia's old
// dynamic-smallest-period sleep -- every job's RunEvery is minutes-to-days,
// so 30s is precise enough; revisit if a job ever needs sub-30s scheduling.
//
//nolint:gochecknoglobals //overridden directly by tests, see jobqueue_internal_test.go
var tickInterval = 30 * time.Second

// jobRunsReader is the slice of repositories.JobRunsRepository JobQueue needs.
type jobRunsReader interface {
	LastSuccessAt(ctx context.Context, jobID string) (*time.Time, error)
}

// JobQueue schedules recurring threading.Job implementations.
type JobQueue struct {
	ctx    context.Context
	pool   *threading.WorkerPool
	repo   jobRunsReader
	mu     sync.RWMutex
	jobs   map[string]*queuedJob
	ticker bool
}

type queuedJob struct {
	job      threading.Job
	callback threading.CallbackFunc
	running  atomic.Bool
	lastRun  atomic.Pointer[time.Time]
}

// NewJobQueue creates a JobQueue backed by db for durable schedule state.
func NewJobQueue(
	ctx context.Context,
	logger *slog.Logger,
	amountWorkers int,
	size int,
	db postgres.DB,
) *JobQueue {
	//nolint:exhaustruct //mu, ticker are zero-value sync.RWMutex/bool
	return &JobQueue{
		ctx:  ctx,
		pool: threading.NewWorkerPool(ctx, logger, amountWorkers, size),
		repo: repositories.NewJobRunsRepository(db),
		jobs: make(map[string]*queuedJob),
	}
}

// AddJob registers a recurring job. It does not run the job immediately --
// the scheduler only runs it once it's due, per global.job_runs.
func (q *JobQueue) AddJob(job threading.Job, callback threading.CallbackFunc) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, ok := q.jobs[job.ID()]; ok {
		return errors.New("a job with this ID already exists")
	}

	last, err := q.repo.LastSuccessAt(q.ctx, job.ID())
	if err != nil {
		return err
	}

	//nolint:exhaustruct //running + lastRun are atomic; zero value is correct
	qj := &queuedJob{job: job, callback: callback}
	qj.lastRun.Store(last)
	q.jobs[job.ID()] = qj

	if !q.ticker {
		q.ticker = true
		go q.scheduleLoop()
	}

	return nil
}

// ForceRun runs the job immediately, bypassing its schedule.
func (q *JobQueue) ForceRun(id string) {
	q.mu.RLock()
	qj, ok := q.jobs[id]
	q.mu.RUnlock()
	if !ok {
		return
	}
	q.runIfNotRunning(qj)
}

// FetchJobIDs returns the IDs of every registered job.
func (q *JobQueue) FetchJobIDs() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	ids := make([]string, 0, len(q.jobs))
	for id := range q.jobs {
		ids = append(ids, id)
	}
	return ids
}

// FetchState reports whether the job is currently running and when it last ran.
func (q *JobQueue) FetchState(id string) (bool, *time.Time) {
	q.mu.RLock()
	qj, ok := q.jobs[id]
	q.mu.RUnlock()
	if !ok {
		return false, nil
	}
	return qj.running.Load(), qj.lastRun.Load()
}

func (q *JobQueue) scheduleLoop() {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			q.tick()
		}
	}
}

func (q *JobQueue) tick() {
	q.mu.RLock()
	due := make([]*queuedJob, 0, len(q.jobs))
	for _, qj := range q.jobs {
		if qj.isDue() {
			due = append(due, qj)
		}
	}
	q.mu.RUnlock()

	for _, qj := range due {
		q.runIfNotRunning(qj)
	}
}

func (qj *queuedJob) isDue() bool {
	if qj.running.Load() {
		return false
	}
	every := qj.job.RunEvery()
	if every <= 0 {
		// RunEvery() <= 0 means trigger-only: the scheduler tick never runs
		// it, only an explicit ForceRun does.
		return false
	}
	last := qj.lastRun.Load()
	return last == nil || time.Since(*last) >= every
}

func (q *JobQueue) runIfNotRunning(qj *queuedJob) {
	if !qj.running.CompareAndSwap(false, true) {
		return
	}
	q.pool.EnqueueWork(func(ctx context.Context, logger *slog.Logger) error {
		defer qj.running.Store(false)

		qj.callback(qj.job.ID(), true, qj.lastRun.Load())

		err := qj.job.Run(ctx, logger)

		now := time.Now().UTC()
		qj.lastRun.Store(&now)
		qj.callback(qj.job.ID(), false, &now)

		return err
	})
}
