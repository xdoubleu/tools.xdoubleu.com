package factory_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/testhelper"
	fhttp "tools.xdoubleu.com/internal/platform/factory/http"
)

const chatRunnerToken = "frt_chat-socket-test"

type chatSocketEnv struct {
	chatEnv
	url    string
	client factoryv1connect.ChatServiceClient
}

func newChatSocketEnv(t *testing.T, admin bool, channel string) chatSocketEnv {
	t.Helper()
	e := newChatEnv(t, 2)
	_, err := testDB.Exec(context.Background(), `
		UPDATE factory.runners SET token_hash = encode(sha256($1::bytea), 'hex')
		WHERE id = $2`, chatRunnerToken, e.runnerID)
	require.NoError(t, err)
	app := newApp(admin, channel)
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	return chatSocketEnv{
		chatEnv: e,
		url:     "ws" + strings.TrimPrefix(ts.URL, "http"),
		client: factoryv1connect.NewChatServiceClient(connect.NewClient(
			connecthttp.NewTransport(http.DefaultClient, ts.URL))),
	}
}

func (e chatSocketEnv) startChat(t *testing.T) *factoryv1.Chat {
	t.Helper()
	res, err := e.client.StartChat(context.Background(), &factoryv1.StartChatRequest{
		RepoId: e.repoID.String(), RunnerId: e.runnerID.String(),
		Harness: factoryv1.Harness_HARNESS_CLAUDE_CODE,
	})
	require.NoError(t, err)
	return res.GetChat()
}

func (e chatSocketEnv) dial(
	t *testing.T,
	path string,
	header http.Header,
) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//nolint:exhaustruct // only the header matters
	conn, _, err := websocket.Dial(
		ctx, e.url+path, &websocket.DialOptions{HTTPHeader: header})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func (e chatSocketEnv) runnerSocket(
	t *testing.T,
	id string,
) *websocket.Conn {
	t.Helper()
	return e.dial(t, "/factory/chats/"+id+"/runner?after=0",
		http.Header{"Authorization": {"Bearer " + chatRunnerToken}})
}

func readFrame(t *testing.T, conn *websocket.Conn) fhttp.ChatFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var frame fhttp.ChatFrame
	require.NoError(t, wsjson.Read(ctx, conn, &frame))
	return frame
}

// readUntilState reads frames up to the next state frame, which ends every
// push, and returns the events before it and the state.
func readUntilState(t *testing.T, conn *websocket.Conn) ([]fhttp.ChatFrame, string) {
	t.Helper()
	var events []fhttp.ChatFrame
	for {
		frame := readFrame(t, conn)
		if frame.Kind == "state" {
			return events, frame.State
		}
		events = append(events, frame)
	}
}

func kinds(frames []fhttp.ChatFrame) []string {
	out := make([]string, len(frames))
	for i, f := range frames {
		out[i] = f.Kind
	}
	return out
}

func writeUpdate(t *testing.T, conn *websocket.Conn, update fhttp.ChatUpdate) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Write(ctx, conn, update))
}

func assistantUpdate(state, text string) fhttp.ChatUpdate {
	var u fhttp.ChatUpdate
	u.State = state
	u.Events = append(u.Events, struct {
		Kind    string `json:"kind"`
		Payload string `json:"payload"`
	}{Kind: "assistant", Payload: text})
	return u
}

func TestAChatStartedByRPCStreamsToTheBrowserAndRunnerSockets(t *testing.T) {
	e := newChatSocketEnv(t, true, constants.ChannelBrowser)
	chat := e.startChat(t)
	assert.Equal(t, "idle", chat.GetState())

	browser := e.dial(t, "/factory/chats/"+chat.GetId()+"/ws", nil)
	events, state := readUntilState(t, browser)
	assert.Empty(t, events)
	assert.Equal(t, "idle", state)
	runner := e.runnerSocket(t, chat.GetId())
	readUntilState(t, runner)

	_, err := e.client.SendChatMessage(context.Background(),
		&factoryv1.SendChatMessageRequest{Id: chat.GetId(), Text: "hello"})
	require.NoError(t, err)

	events, state = readUntilState(t, browser)
	require.Len(t, events, 1)
	assert.Equal(t, "user_message", events[0].Kind)
	assert.Equal(t, "hello", events[0].Payload)
	assert.Equal(t, 1, events[0].Seq)
	assert.Equal(t, "working", state)
	prompt := readFrame(t, runner)
	assert.Equal(t, "hello", prompt.Payload, "the runner gets the person's prompt")

	writeUpdate(t, runner, assistantUpdate("asking", "which branch?"))
	events, state = readUntilState(t, browser)
	assert.Equal(t, []string{"assistant"}, kinds(events))
	assert.Equal(t, "asking", state)

	_, err = e.client.AnswerChat(context.Background(),
		&factoryv1.AnswerChatRequest{Id: chat.GetId(), Text: "main"})
	require.NoError(t, err)
	assert.Equal(t, "user_answer", readFrame(t, runner).Kind,
		"the runner never sees its own events echoed")
}

func TestAReconnectReplaysTheTranscriptFromItsCursor(t *testing.T) {
	e := newChatSocketEnv(t, true, constants.ChannelBrowser)
	chat := e.startChat(t)
	browser := e.dial(t, "/factory/chats/"+chat.GetId()+"/ws", nil)
	readUntilState(t, browser)
	runner := e.runnerSocket(t, chat.GetId())
	readUntilState(t, runner)
	_, err := e.client.SendChatMessage(context.Background(),
		&factoryv1.SendChatMessageRequest{Id: chat.GetId(), Text: "one"})
	require.NoError(t, err)
	readFrame(t, runner)
	writeUpdate(t, runner, assistantUpdate("idle", "two"))
	readUntilState(t, browser)
	readUntilState(t, browser)
	require.NoError(t, browser.Close(websocket.StatusNormalClosure, ""))

	again := e.dial(t, "/factory/chats/"+chat.GetId()+"/ws", nil)
	events, state := readUntilState(t, again)
	assert.Equal(t, []string{"user_message", "assistant"}, kinds(events))
	assert.Equal(t, "idle", state)

	later := e.dial(t, "/factory/chats/"+chat.GetId()+"/ws?after=1", nil)
	events, _ = readUntilState(t, later)
	require.Len(t, events, 1)
	assert.Equal(t, 2, events[0].Seq)

	// A new event reaches the reconnected socket without a gap.
	_, err = e.client.SendChatMessage(context.Background(),
		&factoryv1.SendChatMessageRequest{Id: chat.GetId(), Text: "three"})
	require.NoError(t, err)
	events, _ = readUntilState(t, again)
	require.Len(t, events, 1)
	assert.Equal(t, 3, events[0].Seq)
}

func TestStoppingAChatSendsTheStopToBothSockets(t *testing.T) {
	e := newChatSocketEnv(t, true, constants.ChannelBrowser)
	chat := e.startChat(t)
	browser := e.dial(t, "/factory/chats/"+chat.GetId()+"/ws", nil)
	readUntilState(t, browser)
	runner := e.runnerSocket(t, chat.GetId())
	readUntilState(t, runner)

	_, err := e.client.StopChat(context.Background(),
		&factoryv1.StopChatRequest{Id: chat.GetId(), Report: "done"})
	require.NoError(t, err)

	events, state := readUntilState(t, browser)
	assert.Equal(t, []string{"stopped"}, kinds(events))
	assert.Equal(t, "stopped", state)
	assert.Equal(t, "stopped", readFrame(t, runner).Kind)
}

func TestTheChatSocketsRefuseWhoIsNotTheirPrincipal(t *testing.T) {
	e := newChatSocketEnv(t, true, constants.ChannelBrowser)
	chat := e.startChat(t)
	ctx := context.Background()
	runnerURL := e.url + "/factory/chats/" + chat.GetId() + "/runner"

	for name, header := range map[string]http.Header{
		"no token":      nil,
		"unknown token": {"Authorization": {"Bearer frt_nope"}},
	} {
		//nolint:exhaustruct // only the header matters
		_, res, err := websocket.Dial(ctx, runnerURL,
			&websocket.DialOptions{HTTPHeader: header})
		require.Error(t, err, name)
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode, name)
	}

	_, err := testDB.Exec(ctx,
		`UPDATE factory.runners SET revoked_at = now() WHERE id = $1`, e.runnerID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, `
		INSERT INTO factory.runners (name, created_by, token_hash, paired_at, last_seen)
		VALUES ('Other', 'human:x', encode(sha256('frt_other'::bytea), 'hex'), now(), now())`)
	require.NoError(t, err)
	//nolint:exhaustruct // only the header matters
	_, res, err := websocket.Dial(ctx, runnerURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer frt_other"}},
	})
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, res.StatusCode, "another runner's chat")

	_, res, err = websocket.Dial(ctx, e.url+"/factory/chats/not-an-id/ws", nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	_, res, err = websocket.Dial(ctx,
		e.url+"/factory/chats/00000000-0000-0000-0000-000000000001/ws", nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestTheBrowserSocketIsForAHumanAdminInABrowser(t *testing.T) {
	admin := newChatSocketEnv(t, true, constants.ChannelBrowser)
	chat := admin.startChat(t)

	for name, env := range map[string]chatSocketEnv{
		"plain user": newChatSocketEnvOn(t, admin.chatEnv, false, constants.ChannelBrowser),
		"mcp":        newChatSocketEnvOn(t, admin.chatEnv, true, constants.ChannelMCP),
	} {
		_, res, err := websocket.Dial(context.Background(),
			env.url+"/factory/chats/"+chat.GetId()+"/ws", nil)
		require.Error(t, err, name)
		assert.Equal(t, http.StatusForbidden, res.StatusCode, name)
	}
}

func newChatSocketEnvOn(
	t *testing.T,
	base chatEnv,
	admin bool,
	channel string,
) chatSocketEnv {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(newApp(admin, channel)))
	t.Cleanup(ts.Close)
	return chatSocketEnv{
		chatEnv: base,
		url:     "ws" + strings.TrimPrefix(ts.URL, "http"),
		client: factoryv1connect.NewChatServiceClient(connect.NewClient(
			connecthttp.NewTransport(http.DefaultClient, ts.URL))),
	}
}

func TestARunnerUpdateThatIsRejectedIsAnsweredOnTheSocket(t *testing.T) {
	e := newChatSocketEnv(t, true, constants.ChannelBrowser)
	chat := e.startChat(t)
	runner := e.runnerSocket(t, chat.GetId())
	readUntilState(t, runner)

	update := assistantUpdate("", "forged")
	update.Events[0].Kind = "user_message"
	writeUpdate(t, runner, update)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var reply map[string]any
	require.NoError(t, wsjson.Read(ctx, runner, &reply))
	assert.EqualValues(t, http.StatusBadRequest, reply["status"])

	events, err := e.svc.Chats.Events(ctx, e.admin, uuid.MustParse(chat.GetId()), 0, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestTheChatRPCsAreForAHumanAdminInABrowser(t *testing.T) {
	for name, env := range map[string]chatSocketEnv{
		"plain user": newChatSocketEnv(t, false, constants.ChannelBrowser),
		"mcp":        newChatSocketEnv(t, true, constants.ChannelMCP),
	} {
		_, err := env.client.ListChats(context.Background(), &factoryv1.ListChatsRequest{})
		require.Error(t, err, name)
		assert.NotEqual(t, connect.CodeUnknown, code(err), name)
	}
}

func TestListChatsReturnsTheNewestFirst(t *testing.T) {
	e := newChatSocketEnv(t, true, constants.ChannelBrowser)
	first := e.startChat(t)
	second := e.startChat(t)
	_, err := e.client.StopChat(
		context.Background(), &factoryv1.StopChatRequest{Id: first.GetId()})
	require.NoError(t, err)

	all, err := e.client.ListChats(context.Background(), &factoryv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, all.GetChats(), 2)
	assert.Equal(t, second.GetId(), all.GetChats()[0].GetId())

	active, err := e.client.ListChats(context.Background(),
		&factoryv1.ListChatsRequest{ActiveOnly: true})
	require.NoError(t, err)
	require.Len(t, active.GetChats(), 1)
	assert.Equal(t, second.GetId(), active.GetChats()[0].GetId())
}

func TestChatRPCsRejectABadOrUnknownChatID(t *testing.T) {
	e := newChatSocketEnv(t, true, constants.ChannelBrowser)
	ctx := context.Background()
	unknown := "00000000-0000-0000-0000-000000000000"

	_, err := e.client.SendChatMessage(ctx,
		&factoryv1.SendChatMessageRequest{Id: "nope", Text: "hi"})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = e.client.SendChatMessage(ctx,
		&factoryv1.SendChatMessageRequest{Id: unknown, Text: "hi"})
	require.Error(t, err)
	assert.NotEqual(t, connect.CodeUnknown, code(err))

	_, err = e.client.StopChat(ctx, &factoryv1.StopChatRequest{Id: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = e.client.StopChat(ctx, &factoryv1.StopChatRequest{Id: unknown})
	require.Error(t, err)
	assert.NotEqual(t, connect.CodeUnknown, code(err))

	_, err = e.client.StartChat(ctx, &factoryv1.StartChatRequest{RepoId: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = e.client.StartChat(ctx, &factoryv1.StartChatRequest{RepoId: unknown})
	require.Error(t, err)
	assert.NotEqual(t, connect.CodeUnknown, code(err))
}
