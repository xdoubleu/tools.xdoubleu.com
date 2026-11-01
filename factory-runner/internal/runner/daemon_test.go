package runner_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

type scripted struct {
	calls  int
	answer func(call int) error
	cancel context.CancelFunc
}

func (s *scripted) Heartbeat(_ context.Context, token string) error {
	s.calls++
	if token != "frt_x" {
		return errors.New("wrong token")
	}
	return s.answer(s.calls)
}

func newQuiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newScripted(answer func(int) error, cancel context.CancelFunc) *scripted {
	return &scripted{calls: 0, answer: answer, cancel: cancel}
}

func TestDaemonStopsWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hb := newScripted(func(call int) error {
		if call == 3 {
			cancel()
		}
		return nil
	}, cancel)
	var slept []time.Duration
	err := runner.RunForTest(ctx, hb, "frt_x", newQuiet(), noOpts(),
		func(_ context.Context, d time.Duration) { slept = append(slept, d) },
		time.Now)
	require.NoError(t, err)
	assert.Equal(t, 3, hb.calls)
	assert.NotEmpty(t, slept, "a fast answer isn't repeated at once")
}

func TestDaemonBacksOffThenRecovers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hb := newScripted(func(call int) error {
		switch {
		case call <= 7:
			return errors.New("offline")
		case call == 9:
			cancel()
		}
		return nil
	}, cancel)
	var slept []time.Duration
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(), noOpts(),
		func(_ context.Context, d time.Duration) { slept = append(slept, d) },
		time.Now))

	want := []time.Duration{
		time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second,
		16 * time.Second, 30 * time.Second, 30 * time.Second,
	}
	assert.Equal(t, want, slept[:7])
}

func TestDaemonEndsWhenRevoked(t *testing.T) {
	hb := newScripted(func(int) error {
		return &runner.Error{Code: "unauthenticated", Message: "revoked"}
	}, nil)
	err := runner.RunForTest(context.Background(), hb, "frt_x", newQuiet(),
		noOpts(), func(context.Context, time.Duration) {}, time.Now)
	require.ErrorIs(t, err, runner.ErrRevoked)
	assert.Equal(t, 1, hb.calls)
}

func TestDaemonTreatsAnErrorAfterCancelAsAStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hb := newScripted(func(int) error {
		cancel()
		return errors.New("context canceled")
	}, nil)
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		noOpts(), func(context.Context, time.Duration) {}, time.Now))
}

func TestSleepReturnsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	runner.SleepForTest(ctx, time.Minute)
	assert.Less(t, time.Since(start), time.Second)

	runner.SleepForTest(context.Background(), time.Millisecond)
}

func TestRealRunReturnsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hb := newScripted(func(int) error { cancel(); return nil }, cancel)
	require.NoError(t, runner.Run(ctx, hb, "frt_x", newQuiet(), noOpts()))
}

func noOpts() runner.Options { return runner.Options{Updater: nil, Activity: nil} }

func newFakeUpdater(updated bool, err error) *fakeUpdater {
	return &fakeUpdater{
		calls: atomic.Int32{}, updated: updated, err: err,
		called: make(chan struct{}), once: sync.Once{},
	}
}

type fakeUpdater struct {
	calls   atomic.Int32
	updated bool
	err     error
	called  chan struct{}
	once    sync.Once
}

func (f *fakeUpdater) Update(context.Context, func() bool) (bool, error) {
	f.calls.Add(1)
	f.once.Do(func() { close(f.called) })
	return f.updated, f.err
}

// waitCalled blocks until the update has started, and a moment more so what it
// does on return (a log line) has happened.
func (f *fakeUpdater) waitCalled() {
	<-f.called
	time.Sleep(10 * time.Millisecond)
}

// ticking is a clock that moves a minute per read, so a few loops pass an hour.
func ticking() func() time.Time {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		now = now.Add(time.Minute)
		return now
	}
}

func TestDaemonRestartsAfterASelfUpdate(t *testing.T) {
	hb := newScripted(func(int) error { return nil }, nil)
	upd := newFakeUpdater(true, nil)
	err := runner.RunForTest(context.Background(), hb, "frt_x", newQuiet(),
		runner.Options{Updater: upd, Activity: nil},
		func(context.Context, time.Duration) {}, ticking())
	require.ErrorIs(t, err, runner.ErrRestart)
	assert.Equal(t, 1, int(upd.calls.Load()))
	assert.Positive(t, hb.calls, "heartbeats go on while the update runs")
}

func TestDaemonChecksForUpdatesOncePerInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hb := newScripted(func(call int) error {
		time.Sleep(time.Millisecond) // lets the update goroutine run
		if call == 130 {
			cancel()
		}
		return nil
	}, cancel)
	upd := newFakeUpdater(false, nil)
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		runner.Options{Updater: upd, Activity: nil},
		func(context.Context, time.Duration) {}, ticking()))
	// The clock moves 3-4 minutes per loop: about 130 loops is 6 to 8 hours.
	assert.Greater(t, int(upd.calls.Load()), 2)
	assert.Less(t, int(upd.calls.Load()), 10)
}

func TestDaemonKeepsRunningWhenAnUpdateFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	upd := newFakeUpdater(false, errors.New("no signature"))
	hb := newScripted(func(call int) error {
		if call == 2 {
			upd.waitCalled()
			cancel()
		}
		return nil
	}, cancel)
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		runner.Options{Updater: upd, Activity: nil},
		func(context.Context, time.Duration) {}, ticking()))
	assert.Equal(t, 1, int(upd.calls.Load()))
	assert.Equal(t, 2, hb.calls)
}

func TestDaemonDoesNotUpdateWhileAJobRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hb := newScripted(func(call int) error {
		if call == 3 {
			cancel()
		}
		return nil
	}, cancel)
	upd := newFakeUpdater(true, nil)
	activity := runner.NewActivity(awakeStub())
	require.NoError(t, activity.Begin())
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		runner.Options{Updater: upd, Activity: activity},
		func(context.Context, time.Duration) {}, ticking()))
	assert.Equal(t, 0, int(upd.calls.Load()))
}

// syncBuffer is a log sink the update goroutine and the test can share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestDaemonLogsAFailedUpdateButNotAMissingKey(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		logged bool
	}{
		"no key":         {runner.ErrUpdateSigningUnset, false},
		"a real failure": {errors.New("bad signature"), true},
	} {
		sink := &syncBuffer{mu: sync.Mutex{}, buf: bytes.Buffer{}}
		log := slog.New(slog.NewTextHandler(sink, nil))
		ctx, cancel := context.WithCancel(context.Background())
		upd := newFakeUpdater(false, tc.err)
		hb := newScripted(func(call int) error {
			if call == 3 {
				upd.waitCalled()
				cancel()
			}
			return nil
		}, cancel)
		require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", log,
			runner.Options{Updater: upd, Activity: nil},
			func(context.Context, time.Duration) {}, ticking()))
		assert.Equal(t, tc.logged, sink.String() != "", name)
	}
}
