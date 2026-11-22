package acp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/acp/acptest"
)

// wire joins a client and an agent with two pipes; stop hangs up the agent's input.
type wire struct {
	client *acp.Client
	agent  *acptest.Agent
	events *eventLog
	stop   func()
}

type eventLog struct {
	mu sync.Mutex
	e  []acp.Event
}

func (l *eventLog) add(e acp.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.e = append(l.e, e)
}

func (l *eventLog) all() []acp.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]acp.Event(nil), l.e...)
}

func newWire(t *testing.T, noise string) *wire {
	t.Helper()
	toAgentR, toAgentW := io.Pipe()
	toClientR, toClientW := io.Pipe()
	agent := acptest.New()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if noise != "" {
			_, _ = toClientW.Write([]byte(noise))
		}
		agent.Serve(ctx, toAgentR, toClientW)
		_ = toClientW.Close()
	}()
	log := &eventLog{}
	client := acp.NewClient(toClientR, toAgentW, log.add)
	go func() { _ = client.Serve(ctx) }()
	stop := func() { _ = toAgentW.Close(); cancel() }
	t.Cleanup(stop)
	return &wire{client: client, agent: agent, events: log, stop: stop}
}

func TestAPromptStreamsItsUpdatesAndAllowsEveryPermission(t *testing.T) {
	w := newWire(t, "")
	ctx := context.Background()
	require.NoError(t, w.client.Initialize(ctx))
	id, err := w.client.NewSession(ctx, "/work")
	require.NoError(t, err)

	reason, err := w.client.Prompt(ctx, id, "fix it")
	require.NoError(t, err)
	assert.Equal(t, "end_turn", reason)

	events := w.events.all()
	require.Len(t, events, 3)
	assert.Equal(t, "update", events[0].Kind)
	assert.Equal(t, "agent_message_chunk", events[0].UpdateKind())
	assert.Equal(t, "working on fix it", events[0].Text())
	assert.Equal(t, "permission", events[1].Kind)
	assert.Equal(t, "yes", events[1].Option, "allow once is preferred")
	assert.JSONEq(t, `{"toolCallId":"t1","title":"bash"}`, string(events[1].Tool))
	assert.Equal(t, "done", events[2].Text())

	require.Len(t, w.agent.Outcomes(), 1)
	assert.JSONEq(t, `{"outcome":{"outcome":"selected","optionId":"yes"}}`,
		string(w.agent.Outcomes()[0]))
}

func TestAlwaysIsChosenWhenOnceIsNotOffered(t *testing.T) {
	w := newWire(t, "")
	w.agent.Options = []map[string]string{
		{"optionId": "no", "name": "n", "kind": "reject_always"},
		{"optionId": "always", "name": "a", "kind": "allow_always"},
	}
	ctx := context.Background()
	require.NoError(t, w.client.Initialize(ctx))
	id, err := w.client.NewSession(ctx, "/work")
	require.NoError(t, err)
	_, err = w.client.Prompt(ctx, id, "x")
	require.NoError(t, err)
	assert.JSONEq(t, `{"outcome":{"outcome":"selected","optionId":"always"}}`,
		string(w.agent.Outcomes()[0]))
}

func TestAPermissionWithNoAllowingOptionIsCancelled(t *testing.T) {
	w := newWire(t, "")
	w.agent.Options = []map[string]string{
		{"optionId": "no", "name": "n", "kind": "reject_once"},
	}
	ctx := context.Background()
	require.NoError(t, w.client.Initialize(ctx))
	id, err := w.client.NewSession(ctx, "/work")
	require.NoError(t, err)
	_, err = w.client.Prompt(ctx, id, "x")
	require.NoError(t, err)
	assert.JSONEq(t, `{"outcome":{"outcome":"cancelled"}}`, string(w.agent.Outcomes()[0]))
	assert.Empty(t, w.events.all()[1].Option)
}

func TestCancelEndsTheTurnWithTheCancelledStopReason(t *testing.T) {
	w := newWire(t, "")
	ctx := context.Background()
	require.NoError(t, w.client.Initialize(ctx))
	id, err := w.client.NewSession(ctx, "/work")
	require.NoError(t, err)

	got := make(chan string, 1)
	go func() {
		reason, _ := w.client.Prompt(ctx, id, "wait")
		got <- reason
	}()
	require.Eventually(t, func() bool { return len(w.agent.Prompts()) == 1 },
		time.Second, time.Millisecond)
	require.NoError(t, w.client.Cancel(id))
	select {
	case reason := <-got:
		assert.Equal(t, "cancelled", reason)
	case <-time.After(2 * time.Second):
		t.Fatal("the turn did not end")
	}
}

func TestLinesThatAreNotJSONObjectsAreSkipped(t *testing.T) {
	w := newWire(t, "warming up\n[1,2]\n\n{not json}\n")
	ctx := context.Background()
	require.NoError(t, w.client.Initialize(ctx))
}

func TestACallFailsOnceTheAgentHangsUp(t *testing.T) {
	w := newWire(t, "")
	ctx := context.Background()
	require.NoError(t, w.client.Initialize(ctx))
	id, err := w.client.NewSession(ctx, "/work")
	require.NoError(t, err)
	go func() { _, _ = w.client.Prompt(ctx, id, "wait") }()
	require.Eventually(t, func() bool { return len(w.agent.Prompts()) == 1 },
		time.Second, time.Millisecond)
	w.stop()
	select {
	case <-w.client.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the client did not notice")
	}
	_, err = w.client.Prompt(ctx, id, "again")
	assert.ErrorIs(t, err, acp.ErrClosed)
}

func TestAnErrorAnswerIsReturnedAsAnRPCError(t *testing.T) {
	w := newWire(t, "")
	ctx := context.Background()
	require.NoError(t, w.client.Initialize(ctx))
	// The scripted agent knows no such method; Cancel is a notification, so use a
	// Conn directly.
	toAgentR, toAgentW := io.Pipe()
	toConnR, toConnW := io.Pipe()
	go w.agent.Serve(ctx, toAgentR, toConnW)
	conn := acp.NewConn(toConnR, toAgentW, nil)
	go func() { _ = conn.Serve(ctx) }()
	err := conn.Call(ctx, "nope", nil, nil)
	var rpc *acp.RPCError
	require.ErrorAs(t, err, &rpc)
	assert.Equal(t, -32601, rpc.Code)
	assert.Contains(t, rpc.Error(), "unsupported")
}

func TestACallGivesUpWhenItsContextEnds(t *testing.T) {
	w := newWire(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, w.client.Initialize(ctx))
	id, err := w.client.NewSession(ctx, "/work")
	require.NoError(t, err)
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err = w.client.Prompt(ctx, id, "wait")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestAClientRefusesAnAgentOfAnotherVersion(t *testing.T) {
	toAgentR, toAgentW := io.Pipe()
	toClientR, toClientW := io.Pipe()
	agent := acp.NewConn(toAgentR, toClientW, func(
		context.Context, string, json.RawMessage,
	) (any, error) {
		return map[string]int{"protocolVersion": 99}, nil
	})
	go func() { _ = agent.Serve(context.Background()) }()
	client := acp.NewClient(toClientR, toAgentW, nil)
	go func() { _ = client.Serve(context.Background()) }()
	assert.Error(t, client.Initialize(context.Background()))
}

func TestAClientAnswersAnUnknownRequestWithAnError(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"fs/read_text_file","params":{}}` + "\n")
	client := acp.NewClient(in, &out, nil)
	require.NoError(t, client.Serve(context.Background()))
	assert.Contains(t, out.String(), `"id":7`)
	assert.Contains(t, out.String(), `-32601`)
}

type backend struct {
	opened  string
	emit    func(json.RawMessage)
	openErr error
	prompts []string
	cancels int
	mu      sync.Mutex
}

func (b *backend) Open(_ context.Context, cwd string, emit func(json.RawMessage)) error {
	b.opened, b.emit = cwd, emit
	return b.openErr
}

func (b *backend) Prompt(_ context.Context, text string) (string, error) {
	b.mu.Lock()
	b.prompts = append(b.prompts, text)
	b.mu.Unlock()
	b.emit(json.RawMessage(`{"sessionUpdate":"agent_message_chunk","content":{"text":"hi"}}`))
	if text == "boom" {
		return "", errors.New("boom")
	}
	return "end_turn", nil
}

func (b *backend) Cancel() {
	b.mu.Lock()
	b.cancels++
	b.mu.Unlock()
}

func newServer(t *testing.T, b *backend) (*acp.Client, *eventLog) {
	t.Helper()
	toSrvR, toSrvW := io.Pipe()
	toCliR, toCliW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	srv := acp.NewServer(toSrvR, toCliW, "wardcase", "test", b)
	go func() { _ = srv.Serve(ctx) }()
	log := &eventLog{}
	client := acp.NewClient(toCliR, toSrvW, log.add)
	go func() { _ = client.Serve(ctx) }()
	t.Cleanup(func() { _ = toSrvW.Close(); cancel() })
	return client, log
}

func TestAServerFrontsOneBackendSession(t *testing.T) {
	b := &backend{}
	client, log := newServer(t, b)
	ctx := context.Background()
	require.NoError(t, client.Initialize(ctx))
	id, err := client.NewSession(ctx, "/Users/me/repo")
	require.NoError(t, err)
	assert.Equal(t, "/Users/me/repo", b.opened)

	reason, err := client.Prompt(ctx, id, "do it")
	require.NoError(t, err)
	assert.Equal(t, "end_turn", reason)
	assert.Equal(t, []string{"do it"}, b.prompts)
	require.Len(t, log.all(), 1)
	assert.Equal(t, "hi", log.all()[0].Text())

	_, err = client.NewSession(ctx, "/other")
	assert.ErrorContains(t, err, "one session")

	_, err = client.Prompt(ctx, "someone-else", "x")
	assert.ErrorContains(t, err, "unknown session")

	_, err = client.Prompt(ctx, id, "boom")
	assert.ErrorContains(t, err, "boom")

	require.NoError(t, client.Cancel(id))
	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.cancels == 1
	}, time.Second, time.Millisecond)
}

func TestAServerFlattensEveryKindOfPromptBlock(t *testing.T) {
	b := &backend{}
	client, _ := newServer(t, b)
	ctx := context.Background()
	require.NoError(t, client.Initialize(ctx))
	id, err := client.NewSession(ctx, "/x")
	require.NoError(t, err)
	_, err = client.PromptBlocks(ctx, id, []map[string]any{
		{"type": "text", "text": "a"},
		{"type": "resource", "resource": map[string]string{"uri": "file:///f", "text": "body"}},
		{"type": "resource_link", "uri": "file:///g"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"a\n\nfile:///f:\nbody\n\nfile:///g"}, b.prompts)
}

func TestAServerThatCannotOpenItsSessionAllowsANewTry(t *testing.T) {
	b := &backend{openErr: errors.New("no VM")}
	client, _ := newServer(t, b)
	ctx := context.Background()
	require.NoError(t, client.Initialize(ctx))
	_, err := client.NewSession(ctx, "/x")
	assert.ErrorContains(t, err, "no VM")
	b.openErr = nil
	_, err = client.NewSession(ctx, "/x")
	assert.NoError(t, err)
}
