package runner_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	assert.Equal(t, []int{1, 2},
		[]int{env.api.numbered()[0].Seq, env.api.numbered()[1].Seq})
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
	assert.Equal(t, 102, env.api.numbered()[0].Seq, "numbers survive the drop")
	assert.Len(t, got[len(got)-1], 32*1024, "a long line is cut")
	assert.GreaterOrEqual(t, env.api.batches, 10, "200 lines at a time")
}

func TestSecretsAreRedactedAsJSONHoldsThem(t *testing.T) {
	multi := "-----BEGIN KEY-----\nabc\"def\\ghi<>&\n-----END KEY-----"
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		quoted, _ := json.Marshal(multi) // as Go escapes it
		var plain bytes.Buffer
		enc := json.NewEncoder(&plain)
		enc.SetEscapeHTML(false) // as JSON.stringify does
		_ = enc.Encode(multi)
		_, _ = io.WriteString(spec.Log, `{"a":`+string(quoted)+"}\n")
		_, _ = io.WriteString(spec.Log, `{"b":`+strings.TrimSpace(plain.String())+"}\n")
		return 0, nil
	}, newJob())
	env.runner.Secrets = []string{multi}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{`{"a":"[redacted]"}`, `{"b":"[redacted]"}`}, env.api.logged())
}

func TestALineLongerThanTheBufferIsDroppedWholeAndNulsAreRemoved(t *testing.T) {
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, "before\n")
		// 1.2 MiB with a secret straddling the 1 MiB mark, then its tail.
		_, _ = io.WriteString(spec.Log, strings.Repeat("x", 1<<20-5)+eventSecret[:10])
		_, _ = io.WriteString(spec.Log, eventSecret[10:]+strings.Repeat("y", 200<<10)+"\n")
		_, _ = io.WriteString(spec.Log, "a\x00b\n")
		return 0, nil
	}, newJob())
	env.runner.Secrets = []string{eventSecret}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"before", "ab"}, env.api.logged())
}

func TestLinesAreNotNumberedPastTheJobCap(t *testing.T) {
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		for range 5100 {
			_, _ = io.WriteString(spec.Log, "line\n")
		}
		return 0, nil
	}, newJob())
	env.runner.EventsEvery = 2 * time.Millisecond

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	got := env.api.numbered()
	require.NotEmpty(t, got)
	assert.Equal(t, 5000, got[len(got)-1].Seq, "numbering stops at the job cap")
}

func TestTheJobsOwnAccountSecretNeverLeavesTheMac(t *testing.T) {
	const mcpSecret = "job-account-secret-value"
	job := newJob()
	job.MCPClientID, job.MCPClientSecret = "agent-1", mcpSecret

	// In the log.
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, `{"type":"assistant","text":"`+mcpSecret+`"}`+"\n")
		commitFile(spec.CloneDir, "a.txt", "x")
		return 0, nil
	}, job)
	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{`{"type":"assistant","text":"[redacted]"}`}, env.api.logged())

	// In a commit.
	job = newJob()
	job.MCPClientSecret = mcpSecret
	env = newEnv(t, agentDoes("done", func(clone string) {
		commitFile(clone, "a.txt", mcpSecret)
	}), job)
	got := finish(t, env)
	assert.False(t, got.succeeded)
	assert.Contains(t, got.outcome, "a credential value appears")
	assert.NotContains(t, got.outcome, mcpSecret)
}
