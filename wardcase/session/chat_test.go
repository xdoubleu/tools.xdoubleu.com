package session_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/acp/acptest"
	"tools.xdoubleu.com/wardcase/session"
	"tools.xdoubleu.com/wardcase/vm"
)

// agentBox runs a scripted ACP agent as the harness.
type agentBox struct{ agent *acptest.Agent }

func (b agentBox) Run(context.Context, vm.SandboxSpec) (int, error) { return 0, nil }

func (b agentBox) Open(context.Context, vm.SandboxSpec) (vm.SandboxSession, error) {
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

func TestAChatFromAURLStreamsATurnAndReportsWhatItLeft(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(origin, "a.txt"), nil, 0o600))
	git(t, origin, "add", ".")
	git(t, origin, "commit", "-q", "-m", "init")

	agent := acptest.New()
	r := session.New(session.Options{
		Box: agentBox{agent: agent}, Secrets: secrets{"openrouter": "or"},
		StateDir: t.TempDir(), TempDir: t.TempDir(), Out: nil, Notes: nil, GH: nil,
	})
	var mu sync.Mutex
	var kinds []string
	ctx := context.Background()
	chat, err := r.Start(ctx, session.ChatRequest{
		URL: origin, Base: "main", Branch: "agent/chat", Token: "",
		Harness: session.OpenCode, OpenCodeModel: "vendor/model", Board: nil,
		MCP: nil, Order: nil,
		OnEvent: func(e acp.Event) {
			mu.Lock()
			kinds = append(kinds, e.Kind)
			mu.Unlock()
		},
	})
	require.NoError(t, err)

	reason, err := chat.Prompt(ctx, "hello")
	require.NoError(t, err)
	assert.Equal(t, "end_turn", reason)
	mu.Lock()
	assert.Equal(t, []string{"update", "permission", "update"}, kinds)
	mu.Unlock()

	res := chat.Close(ctx)
	assert.Equal(t, "agent/chat", res.Branch)
	assert.Equal(t, session.OpenCode, res.Harness)
	assert.False(t, res.Left())
	assert.Empty(t, res.Path)
}

func TestAChatOnClaudeIsRefused(t *testing.T) {
	r := session.New(session.Options{
		Box: agentBox{agent: acptest.New()}, Secrets: secrets{"anthropic": "x"},
		StateDir: t.TempDir(), TempDir: t.TempDir(), Out: nil, Notes: nil, GH: nil,
	})
	_, err := r.Start(context.Background(), session.ChatRequest{
		URL: "/nowhere", Base: "main", Branch: "b", Token: "",
		Harness: session.Claude, OpenCodeModel: "", Board: nil, MCP: nil,
		Order: nil, OnEvent: nil,
	})
	require.ErrorContains(t, err, "no ACP server")
}

// seqBox starts the first VM's agent, then the next one's.
type seqBox struct {
	agents []*acptest.Agent
	opened int
}

func (b *seqBox) Run(context.Context, vm.SandboxSpec) (int, error) { return 0, nil }

func (b *seqBox) Open(context.Context, vm.SandboxSpec) (vm.SandboxSession, error) {
	a := b.agents[min(b.opened, len(b.agents)-1)]
	b.opened++
	return agentSession{agent: a}, nil
}

func TestAChatAtItsLimitSwitchesToTheNextHarnessInTheOrder(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(origin, "a.txt"), nil, 0o600))
	git(t, origin, "add", ".")
	git(t, origin, "commit", "-q", "-m", "init")

	limited, next := acptest.New(), acptest.New()
	limited.LimitError = "You've hit your usage limit"
	r := session.New(session.Options{
		Box:      &seqBox{agents: []*acptest.Agent{limited, next}, opened: 0},
		Secrets:  secrets{"openrouter": "or", "gemini": "g"},
		StateDir: t.TempDir(), TempDir: t.TempDir(), Out: nil, Notes: nil, GH: nil,
	})
	ctx := context.Background()
	chat, err := r.Start(ctx, session.ChatRequest{
		URL: origin, Base: "main", Branch: "agent/chat", Token: "",
		Harness: session.OpenCode, OpenCodeModel: "vendor/model", Board: nil,
		MCP: nil, Order: []string{session.OpenCode, session.Gemini}, OnEvent: nil,
	})
	require.NoError(t, err)

	reason, err := chat.Prompt(ctx, "hello")
	require.NoError(t, err)
	assert.Equal(t, session.StopLimited, reason)
	limit, ok := chat.Limit()
	require.True(t, ok)
	assert.Equal(t, session.Gemini, limit.Next)

	require.NoError(t, chat.Switch(ctx, limit.Next, ""))
	_, err = chat.Prompt(ctx, "go on")
	require.NoError(t, err)
	require.Len(t, next.Prompts(), 1)
	assert.Contains(t, next.Prompts()[0], "**User:** hello")
	assert.Equal(t, session.Gemini, chat.Close(ctx).Harness)
}

func TestASwitchToAHarnessWithoutACPFails(t *testing.T) {
	r := session.New(session.Options{
		Box:      &seqBox{agents: []*acptest.Agent{acptest.New()}, opened: 0},
		Secrets:  secrets{"openrouter": "or"},
		StateDir: t.TempDir(), TempDir: t.TempDir(), Out: nil, Notes: nil, GH: nil,
	})
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(origin, "a.txt"), nil, 0o600))
	git(t, origin, "add", ".")
	git(t, origin, "commit", "-q", "-m", "init")
	ctx := context.Background()
	chat, err := r.Start(ctx, session.ChatRequest{
		URL: origin, Base: "main", Branch: "b", Token: "", Harness: session.OpenCode,
		OpenCodeModel: "vendor/model", Board: nil, MCP: nil, Order: nil, OnEvent: nil,
	})
	require.NoError(t, err)
	defer chat.Close(ctx)
	require.ErrorContains(t, chat.Switch(ctx, session.Claude, ""), "no ACP server")
}
