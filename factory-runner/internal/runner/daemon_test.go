package runner_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
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
	err := runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		func(_ context.Context, d time.Duration) { slept = append(slept, d) })
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
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		func(_ context.Context, d time.Duration) { slept = append(slept, d) }))

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
		func(context.Context, time.Duration) {})
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
		func(context.Context, time.Duration) {}))
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
	require.NoError(t, runner.Run(ctx, hb, "frt_x", newQuiet()))
}
