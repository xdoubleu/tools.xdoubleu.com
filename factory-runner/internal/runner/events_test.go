package runner_test

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const eventSecret = "sk-ant-oat01-secret-value"

func TestTheHarnessLogIsForwardedWithSecretsRemoved(t *testing.T) {
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		// A line split over writes, a secret split over writes, a blank line.
		_, _ = io.WriteString(spec.Log, `{"type":"system","session_id":"s"}`+"\n\n")
		_, _ = io.WriteString(spec.Log, `{"type":"assistant","text":"key `+eventSecret[:10])
		_, _ = io.WriteString(spec.Log,
			eventSecret[10:]+` and device tok-device-12345"}`+"\n")
		commitFile(spec.CloneDir, "a.txt", "x")
		_ = writes("done", 0)
		return 0, nil
	}, newJob())
	env.runner.Secrets = []string{eventSecret}
	env.runner.Token = "tok-device-12345"

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.logged()
	require.Len(t, got, 2)
	assert.Equal(t, `{"type":"system","session_id":"s"}`, got[0])
	assert.Equal(t,
		`{"type":"assistant","text":"key [redacted] and device [redacted]"}`, got[1])
}

func TestTheLogIsSentWhileTheJobRuns(t *testing.T) {
	sent := make(chan struct{})
	env := newEnv(t, func(ctx context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, `{"type":"assistant"}`+"\n")
		select {
		case <-sent:
		case <-ctx.Done():
		}
		return 0, nil
	}, newJob())
	env.runner.EventsEvery = 5 * time.Millisecond
	go func() {
		for len(env.api.logged()) == 0 {
			time.Sleep(time.Millisecond)
		}
		close(sent)
	}()
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	select {
	case <-sent:
	default:
		t.Fatal("the line was only sent after the job ended")
	}
}

func TestAFailedSendKeepsTheLinesForTheNextTry(t *testing.T) {
	env := newEnv(t, writes("done", 0), newJob())
	env.api.appendErr = errors.New("server down")
	env.runner.EventsEvery = 5 * time.Millisecond
	// The first tries fail; the final flush after the job finds the server back.
	env.sandbox.run = func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, `{"type":"a"}`+"\n"+`{"type":"b"}`+"\n")
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			env.api.mu.Lock()
			tried := env.api.batches
			env.api.mu.Unlock()
			if tried > 0 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		env.api.mu.Lock()
		env.api.appendErr = nil
		env.api.mu.Unlock()
		return 0, nil
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{`{"type":"a"}`, `{"type":"b"}`}, env.api.logged())
}

func TestAQueueOfUnsentLinesIsBoundedAndSentInBatches(t *testing.T) {
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		for i := range 2100 {
			_, _ = io.WriteString(spec.Log, `{"n":`+strconv.Itoa(i)+`}`+"\n")
		}
		_, _ = io.WriteString(spec.Log, strings.Repeat("x", 40*1024)+"\n")
		return 0, nil
	}, newJob())
	env.runner.EventsEvery = time.Hour // only the final flush sends

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.logged()
	require.Len(t, got, 2000, "the oldest lines are dropped past the bound")
	assert.Equal(t, `{"n":101}`, got[0])
	assert.Len(t, got[len(got)-1], 32*1024, "a long line is cut")
	assert.GreaterOrEqual(t, env.api.batches, 10, "200 lines at a time")
}
