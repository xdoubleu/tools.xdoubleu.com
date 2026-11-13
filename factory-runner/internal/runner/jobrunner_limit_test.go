package runner_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// limitSandbox emits stream lines, optionally commits work and writes an outcome, and
// exits with the given status.
func limitSandbox(
	lines []string,
	outcome string,
	exit int,
) func(context.Context, runner.SandboxSpec) (int, error) {
	return func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		for _, l := range lines {
			_, _ = io.WriteString(spec.Log, l+"\n")
		}
		if outcome != "" {
			commitFile(spec.CloneDir, "agent.txt", "work")
			_ = os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"),
				[]byte(outcome), 0o644)
		}
		return exit, nil
	}
}

func rejectedAt(secs int64) string {
	return fmt.Sprintf(`{"type":"rate_limit_event","rate_limit_info":`+
		`{"status":"rejected","resetsAt":%d,"rateLimitType":"five_hour"}}`, secs)
}

func TestAJobTheClaudeLimitStoppedIsReportedAsLimitedWithItsResetTime(t *testing.T) {
	reset := time.Now().Add(2 * time.Hour).Unix()
	env := newEnv(t, limitSandbox([]string{
		`{"type":"system","subtype":"init","session_id":"sess-1"}`, rejectedAt(reset),
	}, "", 1), newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.True(t, got.limited)
	assert.False(t, got.succeeded)
	assert.Equal(t, time.Unix(reset, 0).UTC(), got.resetsAt)
	assert.Contains(t, got.outcome,
		"Stopped: the Claude usage limit was reached; it resets at")
	assert.Equal(t, "sess-1", got.session)
	assert.Empty(t, got.prURL)
}

func TestALimitWithoutAResetTimeIsStillALimit(t *testing.T) {
	env := newEnv(t, limitSandbox([]string{
		`{"type":"result","is_error":true,"result":"Claude AI usage limit reached"}`,
	}, "", 1), newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.True(t, got.limited)
	assert.True(t, got.resetsAt.IsZero())
	assert.Equal(t, "Stopped: the Claude usage limit was reached.", got.outcome)
}

func TestACleanExitWithNoOutcomeAfterALimitIsStillLimited(t *testing.T) {
	env := newEnv(t, limitSandbox([]string{rejectedAt(time.Now().Add(time.Hour).Unix())},
		"", 0), newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.True(t, env.api.done()[0].limited)
}

func TestARejectionTheAgentWorkedPastToAFinishedOutcomeIsNotAStop(t *testing.T) {
	env := newEnv(t, limitSandbox([]string{rejectedAt(time.Now().Add(time.Hour).Unix())},
		"Did it.\nPR title: Fix it", 0), newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.limited, "the job finished its work")
	assert.True(t, got.succeeded)
}

func TestAWarningOrAnOrdinaryFailureIsNotALimit(t *testing.T) {
	warning := fmt.Sprintf(`{"type":"rate_limit_event","rate_limit_info":`+
		`{"status":"allowed_warning","resetsAt":%d}}`, time.Now().Add(time.Hour).Unix())
	env := newEnv(t, limitSandbox([]string{warning}, "", 1), newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.limited)
	assert.Contains(t, got.outcome, "The agent exited with status 1")
}

func TestAnOpenCodeJobIsNeverReportedAsClaudeLimited(t *testing.T) {
	job := newJob()
	job.Harness, job.Model = runner.HarnessOpenCode, "vendor/model"
	env := newEnv(t, limitSandbox([]string{rejectedAt(time.Now().Add(time.Hour).Unix())},
		"", 1), job)

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.False(t, env.api.done()[0].limited)
}

func TestTheServerIsToldAboutALimitInTheRequestBody(t *testing.T) {
	var body map[string]any
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = nil
		require.NoError(t, json.Unmarshal(raw, &body))
		_, _ = w.Write([]byte(`{}`))
	})
	at := time.Date(2030, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600))

	require.NoError(t, c.CompleteJob(context.Background(), "frt_x", "j1", runner.JobEnd{
		Succeeded: false, Outcome: "stopped", SessionID: "", PRURL: "",
		Limited: true, ResetsAt: at, Blocked: "",
	}))
	assert.Equal(t, true, body["limited"])
	assert.Equal(t, "2030-01-02T02:04:05Z", body["limitResetsAt"], "UTC, RFC3339")

	require.NoError(t, c.CompleteJob(context.Background(), "frt_x", "j1", runner.JobEnd{
		Succeeded: true, Outcome: "done", SessionID: "", PRURL: "",
		Limited: false, ResetsAt: time.Time{}, Blocked: "",
	}))
	assert.NotContains(t, body, "limited", "absent when not limited")
	assert.NotContains(t, body, "limitResetsAt")
}

func TestALimitLineWithoutATrailingNewlineIsStillSeen(t *testing.T) {
	reset := time.Now().Add(time.Hour).Unix()
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		// The CLI's last line, cut off without a newline.
		_, _ = io.WriteString(spec.Log, rejectedAt(reset))
		return 1, nil
	}, newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.True(t, got.limited)
	assert.Equal(t, time.Unix(reset, 0).UTC(), got.resetsAt)
}

func TestASandboxFailureIsNotHiddenBehindAnEarlierRejection(t *testing.T) {
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, rejectedAt(time.Now().Add(time.Hour).Unix())+"\n")
		return -1, errors.New("the VM died")
	}, newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	got := env.api.done()[0]
	assert.False(t, got.limited, "the real error is what the owner needs to see")
	assert.Contains(t, got.outcome, "The sandbox could not run the job: the VM died")
}
