package factory_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/factory"
	fmodels "tools.xdoubleu.com/internal/platform/factory/internal/models"
	fmcp "tools.xdoubleu.com/internal/platform/factory/mcp"
)

// askEnv is a chat whose job runs as the agent account, with the person's client
// and the agent's MCP session on one app.
type askEnv struct {
	chatEnv
	chat    *fmodels.ChatSession
	client  factoryv1connect.ChatServiceClient
	session *mcp.ClientSession
}

func newAskEnv(t *testing.T) askEnv {
	t.Helper()
	e := newChatEnv(t, 1)
	chat, err := e.start(t)
	require.NoError(t, err)

	accounts := newFakeAccounts()
	accounts.accounts[chat.JobID] = &accountOf{
		userID: agentUserID, clientID: "agent-chat", name: "chat", taskID: uuid.Nil,
		active: true, owner: "human:x",
	}
	app := newAppWith(true, constants.ChannelBrowser, factory.WithJobAccounts(accounts))
	return askEnv{
		chatEnv: e, chat: chat,
		client: factoryv1connect.NewChatServiceClient(
			connect.NewClient(newTransport(t, app))),
		session: mcpSessionOn(t, app, asUser(models.RoleAgent)),
	}
}

type askCall struct {
	out map[string]any
	err error
}

// ask calls factory_chat_ask, which blocks until the person answers, beside the
// test.
func (e askEnv) ask(t *testing.T, args map[string]any) <-chan askCall {
	t.Helper()
	done := make(chan askCall, 1)
	go func() {
		out, err := callTool(t, e.session, "factory_chat_ask", args)
		done <- askCall{out: out, err: err}
	}()
	return done
}

// question waits for the chat's question card and returns its seq.
func (e askEnv) question(t *testing.T) int {
	t.Helper()
	seq := 0
	require.Eventually(t, func() bool {
		events, err := e.svc.Chats.Events(context.Background(), e.admin, e.chat.ID, 0, 0)
		require.NoError(t, err)
		for i := range events {
			if events[i].Kind == fmodels.ChatEventAsk {
				seq = events[i].Seq
			}
		}
		return seq > 0
	}, 5*time.Second, 10*time.Millisecond)
	return seq
}

func (e askEnv) state(t *testing.T) fmodels.ChatState {
	t.Helper()
	got, err := e.svc.Chats.Get(context.Background(), e.admin, e.chat.ID)
	require.NoError(t, err)
	return got.State
}

func TestTappingAnOptionOfAChatQuestionContinuesTheAgentWithIt(t *testing.T) {
	e := newAskEnv(t)

	call := e.ask(t, map[string]any{
		"question": "Which of A or B?", "options": []any{"A", "B"},
	})
	seq := e.question(t)
	assert.Equal(t, fmodels.ChatAsking, e.state(t), "the card waits for the person")

	_, err := e.client.AnswerChat(context.Background(), &factoryv1.AnswerChatRequest{
		Id: e.chat.ID.String(), Text: "B", AskId: int32(seq),
	})
	require.NoError(t, err)

	select {
	case got := <-call:
		require.NoError(t, got.err)
		assert.Equal(t, "answered", got.out["status"])
		assert.Equal(t, "B", got.out["answer"])
		assert.InDelta(t, seq, got.out["ask_id"], 0)
	case <-time.After(5 * time.Second):
		t.Fatal("the agent was not continued")
	}
	assert.Equal(t, fmodels.ChatWorking, e.state(t))
}

func TestAnUnansweredChatQuestionIsKeptWaitingForWithItsAskID(t *testing.T) {
	e := newAskEnv(t)
	previous := fmcp.ChatAskWait
	fmcp.ChatAskWait = 30 * time.Millisecond
	t.Cleanup(func() { fmcp.ChatAskWait = previous })

	first := <-e.ask(t, map[string]any{
		"question": "Sure?", "options": []any{"Yes", "No"}, "multi_select": false,
	})
	require.NoError(t, first.err)
	assert.Equal(t, "waiting", first.out["status"])
	seq := e.question(t)
	assert.InDelta(t, seq, first.out["ask_id"], 0)

	_, err := e.client.AnswerChat(context.Background(), &factoryv1.AnswerChatRequest{
		Id: e.chat.ID.String(), Text: "Yes", AskId: int32(seq),
	})
	require.NoError(t, err)
	again := <-e.ask(t, map[string]any{"ask_id": seq})

	require.NoError(t, again.err)
	assert.Equal(t, "answered", again.out["status"])
	assert.Equal(t, "Yes", again.out["answer"])
	events, err := e.svc.Chats.Events(context.Background(), e.admin, e.chat.ID, 0, 0)
	require.NoError(t, err)
	asks := 0
	for i := range events {
		if events[i].Kind == fmodels.ChatEventAsk {
			asks++
		}
	}
	assert.Equal(t, 1, asks, "asking again records no second card")
}

func TestOnlyAChatSessionCanAskTheAgentsQuestion(t *testing.T) {
	e := newAskEnv(t)
	other := mcpSessionAs(t, asUser(models.RoleAgent))

	_, err := callTool(t, other, "factory_chat_ask", map[string]any{
		"question": "A or B?", "options": []any{"A", "B"},
	})

	require.Error(t, err)
	events, err := e.svc.Chats.Events(context.Background(), e.admin, e.chat.ID, 0, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestAChatQuestionNeedsTwoToFourOptions(t *testing.T) {
	e := newAskEnv(t)

	_, err := callTool(t, e.session, "factory_chat_ask", map[string]any{
		"question": "A?", "options": []any{"A"},
	})

	require.Error(t, err)
}
