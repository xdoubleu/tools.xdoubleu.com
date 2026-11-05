package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// funcLoop is a jobLoop made of a function.
type funcLoop func(ctx context.Context) error

func (f funcLoop) Loop(ctx context.Context) error { return f(ctx) }

// blockUntilStopped is a loop that runs until its context ends.
func blockUntilStopped(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func withJobs(e env, build jobsFunc) env {
	e.app.jobs = build
	return e
}

func TestRunClaimsJobsBesideTheHeartbeatAndStopsBothTogether(t *testing.T) {
	e := newEnv(t)
	var gotCfg runner.Config
	var gotToken string
	loopStopped, beatStopped := false, false
	e.app.daemon = func(
		ctx context.Context, _ runner.Heartbeater, _ string, _ runner.Options,
	) error {
		<-ctx.Done()
		beatStopped = true
		return nil
	}
	build := func(_ context.Context, cfg runner.Config, token string) (jobLoop, error) {
		gotCfg, gotToken = cfg, token
		return funcLoop(func(context.Context) error {
			// A revoked token ends the loop; the heartbeat must stop with it.
			loopStopped = true
			return runner.ErrRevoked
		}), nil
	}
	e = withJobs(e, build)

	assert.Equal(t, 1, e.do("run"))

	assert.Contains(t, e.errs.String(), "revoked")
	assert.True(t, loopStopped)
	assert.True(t, beatStopped, "the heartbeat is stopped when the loop ends")
	assert.Equal(t, "frt_stored", gotToken)
	assert.Equal(t, runner.DefaultServer, gotCfg.Server)
}

func TestASelfUpdateStopsTheJobLoopToo(t *testing.T) {
	e := newEnv(t)
	e.app.daemon = func(
		context.Context, runner.Heartbeater, string, runner.Options,
	) error {
		return runner.ErrRestart
	}
	stopped := false
	e = withJobs(e, func(context.Context, runner.Config, string) (jobLoop, error) {
		return funcLoop(func(ctx context.Context) error {
			<-ctx.Done()
			stopped = true
			return nil
		}), nil
	})

	assert.Equal(t, 0, e.do("run"))
	assert.Contains(t, e.out.String(), "restarting")
	assert.True(t, stopped)
}

func TestAMacThatCannotRunJobsFailsRunBeforeAnyHeartbeat(t *testing.T) {
	e := newEnv(t)
	e = withJobs(e, func(context.Context, runner.Config, string) (jobLoop, error) {
		return nil, errors.New("jobs cannot run on this Mac: container is required")
	})

	assert.Equal(t, 1, e.do("run"))
	assert.Contains(t, e.errs.String(), "container is required")
	assert.Empty(t, *e.ran, "no heartbeat claims the Mac is ready when it is not")
}

func TestARunWithoutJobsKeepsTheHeartbeatAlone(t *testing.T) {
	e := newEnv(t)
	assert.Nil(t, e.app.jobs)
	assert.Equal(t, 0, e.do("run"))
	assert.Equal(t, []string{"frt_stored"}, *e.ran)
}

func TestACleanShutdownEndsBothLoopsWithoutAnError(t *testing.T) {
	e := newEnv(t)
	e.app.daemon = func(
		ctx context.Context, _ runner.Heartbeater, _ string, _ runner.Options,
	) error {
		return blockUntilStopped(ctx)
	}
	ctx, stop := context.WithCancel(context.Background())
	e = withJobs(e, func(context.Context, runner.Config, string) (jobLoop, error) {
		return funcLoop(func(ctx context.Context) error {
			stop() // SIGTERM: the context ends
			return blockUntilStopped(ctx)
		}), nil
	})

	assert.Equal(t, 0, e.app.run(ctx, []string{"run"}))
	assert.Empty(t, e.errs.String())
}

func TestConfigSetsTheImageAndTheDirectories(t *testing.T) {
	e := newEnv(t)
	require.Equal(t, 0, e.do("config", "image", "factory-job:v2"))
	require.Equal(t, 0, e.do("config", "data", "/Volumes/ssd/factory"))
	require.Equal(t, 0, e.do("config", "gocache", "/cache/go"))
	require.Equal(t, 0, e.do("config", "npmcache", "/cache/npm"))

	e.out.Reset()
	require.Equal(t, 0, e.do("config", "show"))
	out := e.out.String()
	assert.Contains(t, out, "image: factory-job:v2\n")
	assert.Contains(t, out, "data: /Volumes/ssd/factory\n")
	assert.Contains(t, out, "gocache: /cache/go\n")
	assert.Contains(t, out, "npmcache: /cache/npm\n")
}

func TestConfigRefusesAnImageOrDirectoryThatCouldBeMisread(t *testing.T) {
	e := newEnv(t)
	for _, args := range [][]string{
		{"config", "image", "--privileged"},
		{"config", "image", ""},
		{"config", "image", "a b"},
		{"config", "data", "relative/dir"},
		{"config", "gocache", "/cache/a,b"},
		{"config", "npmcache", "/cache/a\nb"},
	} {
		assert.Equal(t, 1, e.do(args...), args)
	}
}

func TestConfigShowDefaultsToTheStandardImageAndDataDirectory(t *testing.T) {
	e := newEnv(t)
	require.Equal(t, 0, e.do("config", "show"))
	out := e.out.String()
	assert.Contains(t, out, "image: "+runner.DefaultImage+"\n")
	assert.Contains(t, out, "data: "+runner.DefaultDataDir(e.app.home)+"\n")
}
