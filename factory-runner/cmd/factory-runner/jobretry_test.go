package main

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailedJobsAreBuiltAgainWithGrowingPausesUntilTheyRun(t *testing.T) {
	var out bytes.Buffer
	var pauses []time.Duration
	tries := 0
	ran := false
	loop := &retryingJobs{
		kick: nil, control: nil, mu: sync.Mutex{}, live: nil,
		build: func(context.Context) (jobLoop, error) {
			tries++
			if tries < 4 {
				return nil, errors.New("container image pull failed")
			}
			return funcLoop(func(context.Context) error { ran = true; return nil }), nil
		},
		out:   &out,
		sleep: func(_ context.Context, d time.Duration) { pauses = append(pauses, d) },
	}

	require.NoError(t, loop.Loop(context.Background()))

	assert.True(t, ran)
	assert.Equal(t, []time.Duration{
		time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute,
	}, pauses)
	assert.Contains(t, out.String(), "jobs are still off")
	assert.Contains(t, out.String(), "jobs are on")
}

func TestTheRetryPauseIsCappedAndStopsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var pauses []time.Duration
	loop := &retryingJobs{
		kick: nil, control: nil, mu: sync.Mutex{}, live: nil,
		build: func(context.Context) (jobLoop, error) { return nil, errors.New("no") },
		out:   &bytes.Buffer{},
		sleep: func(_ context.Context, d time.Duration) {
			pauses = append(pauses, d)
			if len(pauses) == 8 {
				cancel()
			}
		},
	}

	require.NoError(t, loop.Loop(ctx))

	assert.Equal(t, jobRetryMax, pauses[6])
	assert.Len(t, pauses, 8)
}

func TestSleepForEndsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	sleepFor(ctx, time.Hour)
	assert.Less(t, time.Since(start), time.Minute)
	sleepFor(context.Background(), time.Millisecond)
}
