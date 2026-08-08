package jobqueue

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/threading"
)

type fakeRepo struct {
	mu   sync.Mutex
	last map[string]*time.Time
}

func newFakeRepo() *fakeRepo {
	//nolint:exhaustruct //mu is a zero-value sync.Mutex
	return &fakeRepo{last: make(map[string]*time.Time)}
}

func (r *fakeRepo) LastSuccessAt(_ context.Context, jobID string) (*time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last[jobID], nil
}

type fakeJob struct {
	id       string
	runEvery time.Duration
	calls    *atomicInt
}

func (f fakeJob) ID() string              { return f.id }
func (f fakeJob) RunEvery() time.Duration { return f.runEvery }
func (f fakeJob) Run(_ context.Context, _ *slog.Logger) error {
	f.calls.add(1)
	return nil
}

type atomicInt struct {
	mu sync.Mutex
	n  int
}

func (a *atomicInt) add(d int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.n += d
}

func (a *atomicInt) get() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.n
}

func newTestQueue(t *testing.T, repo jobRunsReader) *JobQueue {
	t.Helper()
	pool := threading.NewWorkerPool(t.Context(), logging.NewNopLogger(), 1, 10)
	//nolint:exhaustruct //mu, ticker are zero-value sync.RWMutex/bool
	return &JobQueue{
		ctx:  t.Context(),
		pool: pool,
		repo: repo,
		jobs: make(map[string]*queuedJob),
	}
}

func noopCallback(_ string, _ bool, _ *time.Time) {}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition never became true")
}

func TestAddJobDoesNotRunImmediately(t *testing.T) {
	q := newTestQueue(t, newFakeRepo())
	calls := new(atomicInt)
	job := fakeJob{id: "job-a", runEvery: time.Hour, calls: calls}

	require.NoError(t, q.AddJob(job, noopCallback))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, calls.get())
}

func TestJobRunsWhenNeverRunBefore(t *testing.T) {
	old := tickInterval
	tickInterval = 10 * time.Millisecond
	t.Cleanup(func() { tickInterval = old })

	q := newTestQueue(t, newFakeRepo())
	calls := new(atomicInt)
	job := fakeJob{id: "job-b", runEvery: time.Hour, calls: calls}

	require.NoError(t, q.AddJob(job, noopCallback))

	waitFor(t, func() bool { return calls.get() >= 1 })
}

func TestJobSkippedWhenRanRecently(t *testing.T) {
	old := tickInterval
	tickInterval = 10 * time.Millisecond
	t.Cleanup(func() { tickInterval = old })

	repo := newFakeRepo()
	recent := time.Now().Add(-time.Minute)
	repo.last["job-c"] = &recent

	q := newTestQueue(t, repo)
	calls := new(atomicInt)
	job := fakeJob{id: "job-c", runEvery: time.Hour, calls: calls}

	require.NoError(t, q.AddJob(job, noopCallback))

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, calls.get())
}

func TestForceRunBypassesSchedule(t *testing.T) {
	repo := newFakeRepo()
	recent := time.Now()
	repo.last["job-d"] = &recent

	q := newTestQueue(t, repo)
	calls := new(atomicInt)
	job := fakeJob{id: "job-d", runEvery: time.Hour, calls: calls}

	require.NoError(t, q.AddJob(job, noopCallback))
	q.ForceRun("job-d")

	waitFor(t, func() bool { return calls.get() >= 1 })
}

func TestFetchJobIDsAndState(t *testing.T) {
	q := newTestQueue(t, newFakeRepo())
	calls := new(atomicInt)
	job := fakeJob{id: "job-e", runEvery: time.Hour, calls: calls}

	require.NoError(t, q.AddJob(job, noopCallback))

	assert.Equal(t, []string{"job-e"}, q.FetchJobIDs())

	running, lastRun := q.FetchState("job-e")
	assert.False(t, running)
	assert.Nil(t, lastRun)

	_, unknownLastRun := q.FetchState("does-not-exist")
	assert.Nil(t, unknownLastRun)
}

func TestAddJobRejectsDuplicateID(t *testing.T) {
	q := newTestQueue(t, newFakeRepo())
	calls := new(atomicInt)
	job := fakeJob{id: "job-f", runEvery: time.Hour, calls: calls}

	require.NoError(t, q.AddJob(job, noopCallback))
	require.Error(t, q.AddJob(job, noopCallback))
}
