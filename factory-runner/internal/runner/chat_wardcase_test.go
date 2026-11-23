package runner_test

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/wardcase/acp/acptest"
)

// agentBox runs a scripted ACP agent as the harness, with no VM.
type agentBox struct{ agent *acptest.Agent }

func (agentBox) Run(context.Context, runner.SandboxSpec) (int, error) { return 0, nil }

func (b agentBox) Open(
	context.Context, runner.SandboxSpec,
) (runner.SandboxSession, error) {
	return agentSession(b), nil
}

type agentSession struct{ agent *acptest.Agent }

func (agentSession) Exec(context.Context, []string, string, io.Writer) (int, error) {
	return 0, nil
}

func (s agentSession) Attach(
	ctx context.Context, _ []string, in io.Reader, out io.Writer, _ bool,
) (int, error) {
	s.agent.Serve(ctx, in, out)
	return 0, ctx.Err()
}

func (agentSession) Close() {}

func TestAChatRunsInWardcaseWithTheBoardAccountAndStreamsTheTurn(t *testing.T) {
	remote := newRemote(t)
	out, err := exec.Command(
		"git", "-C", remote, "symbolic-ref", "--short", "HEAD").Output()
	require.NoError(t, err)

	s := newChatServer(t)
	s.job.DefaultBranch = strings.TrimSpace(string(out))
	s.job.MCPClientID, s.job.MCPClientSecret = "agent-1", "board-secret-value"
	agent := acptest.New()
	r := chatRunner(t, s, newFakeChat(), runner.OpenCode{Model: "vendor/model"})
	r.OpenChat = nil
	r.Sandbox = agentBox{agent: agent}
	r.OpenRouterKey = func() (string, error) { return "or-key", nil }
	r.Git = &runner.Git{
		Dir: r.DataDir, RemoteURL: func(string, string) string { return remote },
		Run: nil, RunWithEnv: nil,
	}
	done := runChat(t, r)
	s.waitConns(1)

	s.say("user_message", "hello")
	var updates []runner.ChatUpdate
	require.Eventually(t, func() bool {
		u, _ := s.snapshot()
		n := 0
		for _, x := range u {
			n += len(x.Events)
		}
		return n >= 3
	}, 5*time.Second, 5*time.Millisecond)
	s.say("stopped", "")
	ended := s.waitEnded()
	require.NoError(t, <-done)

	prompts := agent.Prompts()
	require.Len(t, prompts, 1)
	assert.Contains(t, prompts[0], "factory_chat_ask", "the instructions come first")
	assert.True(t, strings.HasSuffix(prompts[0], "\n\nhello"), prompts[0])
	var kinds []string
	updates, _ = s.snapshot()
	for _, u := range updates {
		for _, e := range u.Events {
			kinds = append(kinds, e.Kind)
		}
	}
	assert.Equal(t, []string{"update", "permission", "update"}, kinds)
	assert.Contains(t, ended[0], "Nothing was left on branch agent/chat-abcd1234")
}

func TestAChatOnASandboxWithoutSessionsEndsWithTheReason(t *testing.T) {
	s := newChatServer(t)
	r := chatRunner(t, s, newFakeChat(), runner.OpenCode{Model: "vendor/model"})
	r.OpenChat = nil
	r.Sandbox = nil

	done := runChat(t, r)

	ended := s.waitEnded()
	require.NoError(t, <-done)
	assert.Contains(t, ended[0], "cannot hold a chat's VM")
}
