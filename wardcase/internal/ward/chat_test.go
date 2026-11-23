package ward_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/acp/acptest"
	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

type events struct {
	mu sync.Mutex
	e  []acp.Event
}

func (l *events) add(e acp.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.e = append(l.e, e)
}

func (l *events) kinds() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, e := range l.e {
		out = append(out, e.Kind)
	}
	return out
}

func (l *events) last() acp.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.e[len(l.e)-1]
}

// chatFixture plays OpenCode's ACP server with a scripted agent.
func newEvents() *events { return &events{mu: sync.Mutex{}, e: nil} }

func chatFixture(t *testing.T) (*sessionFixture, *acptest.Agent) {
	t.Helper()
	sf := newSessionFixture(t)
	agent := acptest.New()
	sf.f.box.stream = func(_ run, in io.Reader, out io.Writer) int {
		agent.Serve(context.Background(), in, out)
		return 0
	}
	return sf, agent
}

func chatRequest(sf *sessionFixture, log *events) ward.ChatRequest {
	return ward.ChatRequest{
		Source:  &ward.Source{URL: sf.origin, Base: "main", Branch: "agent/chat", Token: ""},
		Dir:     "",
		Harness: ward.OpenCode, OpenCodeModel: "", Board: nil, MCP: nil, Order: nil,
		OnEvent: log.add,
	}
}

func TestAChatRunsOpenCodesACPServerAndStreamsAPrompt(t *testing.T) {
	sf, agent := chatFixture(t)
	log := newEvents()
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, log))
	require.NoError(t, err)

	require.Len(t, sf.f.box.runs, 1)
	r := sf.f.box.runs[0]
	assert.False(t, r.tty, "ACP is JSON over a pipe, not a terminal")
	assert.Equal(t, "opencode", r.command[len(r.command)-2])
	assert.Equal(t, "vendor/model", r.command[len(r.command)-1])
	assert.Contains(t, r.command[2], "exec opencode acp")
	want, err := filepath.EvalSymlinks(chat.Path())
	require.NoError(t, err)
	got, err := filepath.EvalSymlinks(r.spec.CloneDir)
	require.NoError(t, err)
	assert.Equal(t, want, got, "the worktree, as git resolves it")
	assert.Equal(t, "agent/chat", chat.Branch())

	reason, err := chat.Prompt(ctx, "add a file")
	require.NoError(t, err)
	assert.Equal(t, "end_turn", reason)
	assert.Equal(t, []string{"add a file"}, agent.Prompts())
	assert.Equal(t, []string{"update", "permission", "update"}, log.kinds())
	assert.Contains(t, sf.f.notes.String(), "allowed a tool call (yes)")
	require.Len(t, agent.Outcomes(), 1)
	assert.Contains(t, string(agent.Outcomes()[0]), `"optionId":"yes"`)

	left, removed := chat.Close(ctx)
	assert.True(t, removed, "nothing was left")
	assert.Equal(t, 0, left.Uncommitted)
	assert.NoDirExists(t, r.spec.CloneDir)
	assert.NoDirExists(t, r.spec.OutcomeDir)
}

func TestClosingAChatReturnsWhatItLeft(t *testing.T) {
	sf, _ := chatFixture(t)
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, newEvents()))
	require.NoError(t, err)
	wip := filepath.Join(chat.Path(), "wip.txt")
	require.NoError(t, os.WriteFile(wip, []byte("x"), 0o600))
	gitIn(t, chat.Path(), "commit", "-q", "--allow-empty", "-m", "work")

	left, removed := chat.Close(ctx)
	assert.False(t, removed)
	assert.Equal(t, 1, left.Uncommitted)
	assert.Equal(t, 1, left.Unpushed)
	assert.DirExists(t, chat.Path())

	again, _ := chat.Close(ctx)
	assert.Equal(t, left, again, "a second close reports the same")
}

func TestAChatCanBeCancelledMidTurn(t *testing.T) {
	sf, agent := chatFixture(t)
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, newEvents()))
	require.NoError(t, err)
	defer chat.Close(ctx)

	got := make(chan string, 1)
	go func() {
		reason, _ := chat.Prompt(ctx, "wait")
		got <- reason
	}()
	require.Eventually(t, func() bool { return len(agent.Prompts()) == 1 },
		secondish, tick)
	require.NoError(t, chat.Cancel())
	assert.Equal(t, "cancelled", <-got)
}

func TestAChatOnAHarnessWithoutACPIsRefused(t *testing.T) {
	sf, _ := chatFixture(t)
	req := chatRequest(sf, newEvents())
	req.Harness = ward.Claude
	_, err := sf.f.r.OpenChat(context.Background(), req)
	require.ErrorContains(t, err, "no ACP server")
	assert.Empty(t, sf.f.box.runs)
	assert.False(t, ward.HasACP(ward.Claude))
	assert.True(t, ward.HasACP(ward.Codex))
	assert.True(t, ward.HasACP(ward.Gemini))
	assert.True(t, ward.HasACP(ward.OpenCode))
	assert.False(t, ward.HasACP("vim"))
}

func TestAChatWhoseServerDiesBeforeTheHandshakeSaysSo(t *testing.T) {
	sf, _ := chatFixture(t)
	sf.f.box.stream = func(_ run, _ io.Reader, out io.Writer) int {
		_, _ = io.WriteString(out, "opencode: command not found\n")
		return 127
	}
	_, err := sf.f.r.OpenChat(context.Background(), chatRequest(sf, newEvents()))
	require.ErrorContains(t, err, "ACP")
	assert.NoDirExists(t, filepath.Join(sf.f.r.StateDir, "worktrees"))
}

func TestAChatNeedsAVMThatStarts(t *testing.T) {
	sf, _ := chatFixture(t)
	sf.f.box.openErr = io.ErrClosedPipe
	_, err := sf.f.r.OpenChat(context.Background(), chatRequest(sf, newEvents()))
	require.ErrorContains(t, err, "start the VM")
}

func TestAChatFromADirectoryGetsAWorktreeOfItsCheckout(t *testing.T) {
	sf, _ := chatFixture(t)
	req := chatRequest(sf, newEvents())
	req.Source, req.Dir = nil, sf.origin
	chat, err := sf.f.r.OpenChat(context.Background(), req)
	require.NoError(t, err)
	defer chat.Close(context.Background())
	assert.True(t, strings.HasPrefix(chat.Branch(), "wardcase/"))
	assert.NotEqual(t, sf.origin, chat.Path())
}

func TestAChatNeedsItsModelAndSecret(t *testing.T) {
	sf, _ := chatFixture(t)
	req := chatRequest(sf, newEvents())
	sf.f.r.Config.OpenCode.Model = ""
	_, err := sf.f.r.OpenChat(context.Background(), req)
	require.ErrorContains(t, err, "opencode.model")
	req.OpenCodeModel = "other/model"
	chat, err := sf.f.r.OpenChat(context.Background(), req)
	require.NoError(t, err)
	defer chat.Close(context.Background())
	cmd := sf.f.box.runs[0].command
	assert.Equal(t, "other/model", cmd[len(cmd)-1])
}

const (
	secondish = 2 * time.Second
	tick      = time.Millisecond
)

func TestAChatOnCodexAndGeminiRunsTheirACPServersWithKeysOnTheHost(t *testing.T) {
	for _, tc := range []struct{ harness, secret, cmd string }{
		{ward.Codex, ward.SecretOpenAI, "exec codex-acp"},
		{ward.Gemini, ward.SecretGemini, "exec gemini --acp"},
	} {
		t.Run(tc.harness, func(t *testing.T) {
			sf, _ := chatFixture(t)
			sf.f.r.Secrets = memSecrets{tc.secret: "host-key"}
			var proxied vm.EgressConfig
			sf.f.r.NewProxy = func(cfg vm.EgressConfig) (vm.ProxyListener, error) {
				proxied = cfg
				return vm.NewEgressProxy(cfg)
			}
			req := chatRequest(sf, newEvents())
			req.Harness = tc.harness
			ctx := context.Background()

			chat, err := sf.f.r.OpenChat(ctx, req)
			require.NoError(t, err)
			defer chat.Close(ctx)

			r := sf.f.box.runs[0]
			assert.Contains(t, r.command[2], tc.cmd)
			assert.Equal(t, tc.harness, filepath.Base(r.spec.SessionDir))
			for k, v := range r.spec.Env {
				assert.NotContains(t, v, "host-key", k)
			}
			assert.True(t, proxied.OpenAIKey != nil || proxied.GeminiKey != nil)
			reason, err := chat.Prompt(ctx, "hi")
			require.NoError(t, err)
			assert.Equal(t, "end_turn", reason)
		})
	}
}

func TestAGeminiChatNeedsItsSecret(t *testing.T) {
	sf, _ := chatFixture(t)
	req := chatRequest(sf, newEvents())
	req.Harness = ward.Gemini
	_, err := sf.f.r.OpenChat(context.Background(), req)
	require.ErrorIs(t, err, ward.ErrNoSecret)
}

func TestAChatGetsTheBoardAndExtraMCPServersThroughTheProxy(t *testing.T) {
	sf, _ := chatFixture(t)
	var proxied vm.EgressConfig
	sf.f.r.NewProxy = func(cfg vm.EgressConfig) (vm.ProxyListener, error) {
		proxied = cfg
		return vm.NewEgressProxy(cfg)
	}
	req := chatRequest(sf, newEvents())
	req.Board = &ward.Board{
		Server:   "https://board.example",
		Token:    func(context.Context) (string, error) { return "tok", nil },
		Rejected: nil,
	}
	req.MCP = []vm.MCPServer{{
		Name: "docs", URL: "https://docs.example/mcp", Rejected: nil,
		Token: func(context.Context) (string, error) { return "t", nil },
	}}
	ctx := context.Background()

	chat, err := sf.f.r.OpenChat(ctx, req)
	require.NoError(t, err)
	defer chat.Close(ctx)

	env := sf.f.box.runs[0].spec.Env
	assert.Equal(t, "https://board.example", proxied.Server)
	assert.NotNil(t, proxied.MCPToken)
	assert.NotEqual(t, vm.MCPOff, env[vm.MCPEnv])
	assert.Equal(t, "docs", env[vm.MCPServersEnv])
}

func TestAChatWithoutABoardHasNone(t *testing.T) {
	sf, _ := chatFixture(t)
	ctx := context.Background()

	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, newEvents()))
	require.NoError(t, err)
	defer chat.Close(ctx)

	assert.Equal(t, vm.MCPOff, sf.f.box.runs[0].spec.Env[vm.MCPEnv])
}

// limitFixture plays OpenCode at its usage limit, then Codex as a fresh agent.
func limitFixture(
	t *testing.T, limit string,
) (*sessionFixture, *acptest.Agent) {
	t.Helper()
	sf := newSessionFixture(t)
	sf.f.r.Config.Order = []string{ward.Claude, ward.OpenCode, ward.Codex}
	sf.f.r.Secrets = memSecrets{ward.SecretOpenRouter: "k", ward.SecretOpenAI: "host-key"}
	limited, next := acptest.New(), acptest.New()
	limited.LimitError = limit
	sf.f.box.stream = func(_ run, in io.Reader, out io.Writer) int {
		agent := limited
		if len(sf.f.box.runs) > 1 {
			agent = next
		}
		agent.Serve(context.Background(), in, out)
		return 0
	}
	return sf, next
}

func TestALimitOffersTheNextACPHarnessAndTheSwitchCarriesASummary(t *testing.T) {
	sf, next := limitFixture(t, "You've hit your usage limit")
	log := newEvents()
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, log))
	require.NoError(t, err)
	defer chat.Close(ctx)
	_, limited := chat.Limit()
	assert.False(t, limited)

	reason, err := chat.Prompt(ctx, "add a file")
	require.NoError(t, err)
	assert.Equal(t, ward.StopLimited, reason)
	limit, limited := chat.Limit()
	require.True(t, limited)
	assert.Equal(t, ward.OpenCode, limit.Harness)
	assert.Equal(t, ward.Codex, limit.Next, "claude has no ACP server; codex is next")
	assert.Equal(t, []string{"limited"}, log.kinds())

	worktree := chat.Path()
	require.NoError(t, chat.Switch(ctx, ward.Codex, ""))
	assert.Equal(t, []string{"limited", "switch"}, log.kinds())
	assert.Equal(t, ward.Codex, log.last().Harness)
	_, limited = chat.Limit()
	assert.False(t, limited)
	require.Len(t, sf.f.box.runs, 2)
	assert.Contains(t, sf.f.box.runs[1].command[2], "exec codex-acp")
	got, err := filepath.EvalSymlinks(sf.f.box.runs[1].spec.CloneDir)
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(worktree)
	require.NoError(t, err)
	assert.Equal(t, want, got, "the same worktree")

	reason, err = chat.Prompt(ctx, "carry on")
	require.NoError(t, err)
	assert.Equal(t, "end_turn", reason)
	require.Len(t, next.Prompts(), 1)
	first := next.Prompts()[0]
	assert.Contains(t, first, "# Handoff from opencode to codex")
	assert.Contains(t, first, "**User:** add a file")
	assert.True(t, strings.HasSuffix(first, "carry on"))

	_, err = chat.Prompt(ctx, "again")
	require.NoError(t, err)
	assert.Equal(t, "again", next.Prompts()[1], "only the first prompt is seeded")
}

func TestALimitErrorCarriesTheResetTime(t *testing.T) {
	reset := time.Now().Add(time.Hour).Truncate(time.Second)
	sf, _ := limitFixture(t,
		fmt.Sprintf("Claude AI usage limit reached|%d", reset.Unix()))
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, newEvents()))
	require.NoError(t, err)
	defer chat.Close(ctx)

	_, err = chat.Prompt(ctx, "hi")
	require.NoError(t, err)
	limit, _ := chat.Limit()
	assert.True(t, reset.Equal(limit.Resets))
}

func TestWithNoHarnessLeftTheLimitOffersNone(t *testing.T) {
	sf, _ := limitFixture(t, "insufficient credits")
	sf.f.r.Config.Order = []string{ward.Claude, ward.OpenCode}
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, newEvents()))
	require.NoError(t, err)
	defer chat.Close(ctx)

	reason, err := chat.Prompt(ctx, "hi")
	require.NoError(t, err)
	assert.Equal(t, ward.StopLimited, reason)
	limit, _ := chat.Limit()
	assert.Empty(t, limit.Next)
}

func TestAnotherKindOfErrorIsNotALimit(t *testing.T) {
	sf, _ := limitFixture(t, "boom")
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, newEvents()))
	require.NoError(t, err)
	defer chat.Close(ctx)

	_, err = chat.Prompt(ctx, "hi")
	require.ErrorContains(t, err, "boom")
	_, limited := chat.Limit()
	assert.False(t, limited)
}

func TestSwitchingToAHarnessWithoutACPIsRefused(t *testing.T) {
	sf, _ := limitFixture(t, "")
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, newEvents()))
	require.NoError(t, err)
	defer chat.Close(ctx)

	require.ErrorContains(t, chat.Switch(ctx, ward.Claude, ""), "no ACP server")
	assert.Len(t, sf.f.box.runs, 1)
}

func TestASwitchThatCannotStartTheNextHarnessIsAnError(t *testing.T) {
	sf, _ := limitFixture(t, "")
	ctx := context.Background()
	chat, err := sf.f.r.OpenChat(ctx, chatRequest(sf, newEvents()))
	require.NoError(t, err)
	defer chat.Close(ctx)

	sf.f.r.Secrets = memSecrets{}
	require.Error(t, chat.Switch(ctx, ward.Gemini, ""))
}
