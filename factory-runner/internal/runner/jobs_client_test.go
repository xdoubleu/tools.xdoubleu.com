package runner_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestClaimJobReadsAJobOrNone(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/factory.v1.RunnerService/ClaimJob", r.URL.Path)
		assert.Equal(t, "Bearer frt_x", r.Header.Get("Authorization"))
		_, _ = w.Write(
			[]byte(`{"job":{"id":"j1","taskId":"t1","repoOwner":"o","repoName":"n",` +
				`"defaultBranch":"main","harness":"HARNESS_CLAUDE_CODE","prompt":"p",` +
				`"branch":"agent/x","setupCommands":"make","timeLimitMinutes":45,` +
				`"mcpClientId":"agent-1","mcpClientSecret":"s3cret-value"}}`),
		)
	})
	job, err := c.ClaimJob(context.Background(), "frt_x")
	require.NoError(t, err)
	assert.Equal(t, &runner.ClaimedJob{
		ID: "j1", TaskID: "t1", RepoOwner: "o", RepoName: "n", DefaultBranch: "main",
		Harness: runner.HarnessClaudeCode, Prompt: "p", Branch: "agent/x",
		SetupCommands: "make", TimeLimitMinutes: 45,
		MCPClientID: "agent-1", MCPClientSecret: "s3cret-value",
	}, job)

	none := serve(
		t,
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) },
	)
	job, err = none.ClaimJob(context.Background(), "frt_x")
	require.NoError(t, err)
	assert.Nil(t, job)

	revoked := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"unauthenticated","message":"x"}`))
	})
	_, err = revoked.ClaimJob(context.Background(), "frt_x")
	assert.True(t, runner.Unauthenticated(err))
}

func TestCheckInSendsTheSessionAndReadsStop(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/factory.v1.RunnerService/CheckIn", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.JSONEq(t, `{"jobId":"j1","sessionId":"s1"}`, string(body))
		_, _ = w.Write([]byte(`{"stop":true}`))
	})
	stop, err := c.CheckIn(context.Background(), "frt_x", "j1", "s1")
	require.NoError(t, err)
	assert.True(t, stop)

	quiet := serve(
		t,
		func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) },
	)
	stop, err = quiet.CheckIn(context.Background(), "frt_x", "j1", "")
	require.NoError(t, err)
	assert.False(t, stop)
}

func TestCompleteJobSendsTheResult(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/factory.v1.RunnerService/CompleteJob", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.JSONEq(t,
			`{"jobId":"j1","succeeded":true,"outcome":"done","sessionId":"s1"}`, string(body))
		_, _ = w.Write([]byte(`{}`))
	})
	require.NoError(
		t,
		c.CompleteJob(context.Background(), "frt_x", "j1", true, "done", "s1"),
	)

	late := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write(
			[]byte(`{"code":"already_exists","message":"the job is no longer running"}`),
		)
	})
	require.Error(t, late.CompleteJob(context.Background(), "frt_x", "j1", true, "", ""))
}

func TestClaudeCodeRunsPrintModeWithoutTheAgentSDKOrAPromptInArgv(t *testing.T) {
	cmd := runner.ClaudeCode{}.Command()
	assert.Equal(t, "claude", cmd[0])
	assert.Contains(t, cmd, "-p")
	assert.Contains(t, cmd, "stream-json")
	assert.Contains(t, cmd, "bypassPermissions")
}

func TestClaudeCodeReadsTheSessionIDFromItsStream(t *testing.T) {
	h := runner.ClaudeCode{}
	assert.Equal(t, "abc", h.SessionID([]byte(`{"type":"system","session_id":"abc"}`)))
	assert.Empty(t, h.SessionID([]byte(`{"type":"assistant"}`)))
	assert.Empty(t, h.SessionID([]byte(`not json with "session_id" in it`)))
	assert.Empty(t, h.SessionID(nil))
}

func TestGitHubURL(t *testing.T) {
	assert.Equal(t, "https://github.com/o/n.git", runner.GitHubURL("o", "n"))
}

func TestAppendJobEventsSendsTheLinesWithTheDeviceToken(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/factory.v1.RunnerService/AppendJobEvents", r.URL.Path)
		assert.Equal(t, "Bearer frt_x", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		assert.JSONEq(t,
			`{"jobId":"j1","lines":[{"seq":4,"line":"a"},{"seq":5,"line":"b"}]}`, string(body))
		_, _ = w.Write([]byte(`{}`))
	})
	lines := []runner.EventLine{{Seq: 4, Line: "a"}, {Seq: 5, Line: "b"}}
	err := c.AppendJobEvents(context.Background(), "frt_x", "j1", lines)
	require.NoError(t, err)
}
