package runner_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const (
	chatID = "cccccccc-2222-3333-4444-555555555555"
	token  = "frt_0123456789abcdef"
)

// chatServer is the server's side of one chat: the Connect calls a runner makes
// for it and its runner socket.
type chatServer struct {
	t   *testing.T
	srv *httptest.Server
	job *runner.ClaimedJob

	mu       sync.Mutex
	frames   []runner.ChatFrame
	conn     *websocket.Conn
	afters   []int
	updates  []runner.ChatUpdate
	ended    []string
	endedAt  chan struct{}
	claimed  bool
	stop     bool
	dialCode int
	conns    int
}

//nolint:exhaustruct // the server starts empty
func newChatServer(t *testing.T) *chatServer {
	t.Helper()
	s := &chatServer{t: t, endedAt: make(chan struct{}, 4)}
	//nolint:exhaustruct // a chat job has these only
	s.job = &runner.ClaimedJob{
		ID: jobID, ChatID: chatID, RepoOwner: "xdoubleu", RepoName: "tools.xdoubleu.com",
		DefaultBranch: "main", Branch: "agent/chat-abcd1234",
	}
	mux := http.NewServeMux()
	rpc := "POST /factory.v1.RunnerService/"
	mux.HandleFunc(rpc+"ClaimJob", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		resp := map[string]any{}
		if !s.claimed {
			s.claimed = true
			resp["job"] = s.job
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc(rpc+"CheckIn", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]bool{"stop": s.stop})
	})
	mux.HandleFunc(rpc+"GetPushToken", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"token":"ghs_clone"}`)
	})
	mux.HandleFunc(rpc+"EndChat", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ChatID string `json:"chatId"`
			Report string `json:"report"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, s.job.ChatID, req.ChatID)
		s.mu.Lock()
		s.ended = append(s.ended, req.Report)
		s.mu.Unlock()
		s.endedAt <- struct{}{}
		_, _ = io.WriteString(w, `{}`)
	})
	mux.HandleFunc("GET /factory/chats/{id}/runner", s.runnerSocket)
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *chatServer) runnerSocket(w http.ResponseWriter, r *http.Request) {
	assert.Equal(s.t, "Bearer "+token, r.Header.Get("Authorization"))
	assert.Equal(s.t, chatID, r.PathValue("id"))
	s.mu.Lock()
	code := s.dialCode
	s.mu.Unlock()
	if code != 0 {
		http.Error(w, "no", code)
		return
	}
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	conn, acceptErr := websocket.Accept(w, r, nil)
	if acceptErr != nil {
		return
	}
	s.mu.Lock()
	s.conns++
	s.afters = append(s.afters, after)
	s.conn = conn
	for _, f := range s.frames {
		if f.Seq > after {
			_ = wsjson.Write(r.Context(), conn, f)
		}
	}
	// A push may repeat what the replay sent.
	if len(s.frames) > 0 {
		_ = wsjson.Write(r.Context(), conn, s.frames[len(s.frames)-1])
	}
	_ = wsjson.Write(r.Context(), conn,
		runner.ChatFrame{Seq: 0, Kind: "state", Payload: "", State: "idle"})
	s.mu.Unlock()
	for {
		var u runner.ChatUpdate
		if err := wsjson.Read(r.Context(), conn, &u); err != nil {
			return
		}
		s.mu.Lock()
		s.updates = append(s.updates, u)
		s.mu.Unlock()
	}
}

// say is the person writing: kind is user_message, user_answer or stopped.
func (s *chatServer) say(kind, payload string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := runner.ChatFrame{
		Seq: len(s.frames) + 1, Kind: kind, Payload: payload, State: "",
	}
	s.frames = append(s.frames, f)
	if s.conn != nil {
		_ = wsjson.Write(context.Background(), s.conn, f)
	}
}

func (s *chatServer) dropSocket() {
	s.mu.Lock()
	c := s.conn
	s.mu.Unlock()
	if c != nil {
		_ = c.CloseNow()
	}
}

func (s *chatServer) snapshot() ([]runner.ChatUpdate, []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]runner.ChatUpdate(nil), s.updates...), append([]int(nil), s.afters...)
}

func (s *chatServer) waitUpdates(n int) []runner.ChatUpdate {
	s.t.Helper()
	require.Eventually(s.t, func() bool {
		u, _ := s.snapshot()
		return len(u) >= n
	}, 5*time.Second, 5*time.Millisecond)
	u, _ := s.snapshot()
	return u
}

func (s *chatServer) waitConns(n int) {
	s.t.Helper()
	require.Eventually(s.t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.conns >= n
	}, 5*time.Second, 5*time.Millisecond)
}

func (s *chatServer) waitEnded() []string {
	s.t.Helper()
	select {
	case <-s.endedAt:
	case <-time.After(5 * time.Second):
		s.t.Fatal("the runner never ended the chat")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ended...)
}

// fakeChat is a wardcase session that answers a prompt with one update.
type fakeChat struct {
	mu       sync.Mutex
	prompts  []string
	spec     runner.ChatSpec
	hold     chan struct{}
	started  chan struct{}
	canceled bool
	closed   int
	left     runner.ChatLeft
	fail     error
}

//nolint:exhaustruct // the session starts empty
func newFakeChat() *fakeChat {
	return &fakeChat{
		started: make(chan struct{}, 8),
		left:    runner.ChatLeft{Branch: "agent/chat-abcd1234"},
	}
}

func (f *fakeChat) open(
	_ context.Context,
	spec runner.ChatSpec,
) (runner.ChatSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spec = spec
	if f.fail != nil {
		return nil, f.fail
	}
	return f, nil
}

func (f *fakeChat) Prompt(ctx context.Context, text string) (string, error) {
	f.mu.Lock()
	f.prompts = append(f.prompts, text)
	spec, hold := f.spec, f.hold
	f.mu.Unlock()
	f.started <- struct{}{}
	spec.OnEvent("update",
		`{"sessionUpdate":"agent_message_chunk","text":"re: `+text+` `+token+`"}`)
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
		}
	}
	return "end_turn", nil
}

func (f *fakeChat) Cancel() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.canceled = true
	if f.hold != nil {
		close(f.hold)
		f.hold = nil
	}
	return nil
}

func (f *fakeChat) Close(context.Context) runner.ChatLeft {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return f.left
}

func (f *fakeChat) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.prompts...)
}

//nolint:exhaustruct // a chat needs few of the runner's settings
func chatRunner(
	t *testing.T, s *chatServer, f *fakeChat, harness runner.Harness,
) *runner.JobRunner {
	t.Helper()
	return &runner.JobRunner{
		API: runner.NewClient(s.srv.URL, "test"), Token: token, Harness: harness,
		Allowed: func() ([]string, error) {
			return []string{"xdoubleu/tools.xdoubleu.com"}, nil
		},
		MCP: func() ([]runner.MCPServer, []string, error) {
			return []runner.MCPServer{{
				Name: "docs", URL: "https://docs.example/mcp", Rejected: nil,
				Token: func(context.Context) (string, error) { return "t", nil },
			}}, nil, nil
		},
		OpenChat: f.open, DataDir: t.TempDir(), CheckInEvery: 20 * time.Millisecond,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func runChat(t *testing.T, r *runner.JobRunner) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := r.Next(context.Background())
		done <- err
	}()
	return done
}

func TestAChatBridgesTheSocketToTheSessionAndReportsWhatItLeft(t *testing.T) {
	s := newChatServer(t)
	f := newFakeChat()
	f.left = runner.ChatLeft{
		Branch: "agent/chat-abcd1234", Path: "/w/x", Uncommitted: 2, Unpushed: 1,
	}
	r := chatRunner(t, s, f, runner.OpenCode{Model: "vendor/model"})
	done := runChat(t, r)
	s.waitConns(1)

	s.say("user_message", "hello")
	updates := s.waitUpdates(1)
	assert.Equal(t, "idle", updates[len(updates)-1].State)
	s.say("stopped", "bye")
	ended := s.waitEnded()
	require.NoError(t, <-done)

	assert.Equal(t, []string{"hello"}, f.seen(), "the repeated frame is handled once")
	var events []runner.ChatEvent
	for _, u := range updates {
		events = append(events, u.Events...)
	}
	require.Len(t, events, 1)
	assert.Equal(t, "update", events[0].Kind)
	assert.Contains(t, events[0].Payload, "re: hello")
	assert.NotContains(t, events[0].Payload, token, "the device token is redacted")
	require.Len(t, ended, 1)
	assert.Contains(t, ended[0], "2 uncommitted file(s) and 1 unpushed commit(s) in /w/x")
	assert.NotContains(t, ended[0], "runner ended")
	assert.Equal(t, 1, f.closed)
	assert.Equal(t, "vendor/model", f.spec.Model)
	assert.Equal(t, "ghs_clone", f.spec.Token)
	assert.Equal(t, "agent/chat-abcd1234", f.spec.Job.Branch)
	require.Len(t, f.spec.MCP, 1, "the device's MCP servers come with the chat")
	assert.Empty(t, r.RunningJobs())
}

func TestAChatReconnectsFromTheLastSeqItSaw(t *testing.T) {
	s := newChatServer(t)
	f := newFakeChat()
	r := chatRunner(t, s, f, runner.OpenCode{Model: "vendor/model"})
	done := runChat(t, r)
	s.waitConns(1)
	s.say("user_message", "one")
	s.waitUpdates(1)

	s.dropSocket()
	s.waitConns(2)
	s.say("user_message", "two")
	s.waitUpdates(2)
	s.say("stopped", "")
	s.waitEnded()
	require.NoError(t, <-done)

	_, afters := s.snapshot()
	assert.Equal(t, []int{0, 1}, afters[:2], "the second socket asks after message 1")
	assert.Equal(t, []string{"one", "two"}, f.seen())
}

func TestAStopDuringATurnCancelsItBeforeClosing(t *testing.T) {
	s := newChatServer(t)
	f := newFakeChat()
	f.hold = make(chan struct{})
	r := chatRunner(t, s, f, runner.OpenCode{Model: "vendor/model"})
	done := runChat(t, r)
	s.waitConns(1)

	s.say("user_message", "long job")
	<-f.started
	s.say("stopped", "")
	s.waitEnded()
	require.NoError(t, <-done)

	assert.True(t, f.canceled)
	assert.Equal(t, 1, f.closed)
}

func TestAChatTheServerSaysIsOverEndsWithTheCheckIn(t *testing.T) {
	s := newChatServer(t)
	f := newFakeChat()
	r := chatRunner(t, s, f, runner.OpenCode{Model: "vendor/model"})
	done := runChat(t, r)
	s.waitConns(1)

	s.mu.Lock()
	s.stop = true
	s.mu.Unlock()
	s.waitEnded()
	require.NoError(t, <-done)

	assert.Equal(t, 1, f.closed)
}

func TestAChatTheServerNoLongerListsEnds(t *testing.T) {
	s := newChatServer(t)
	s.dialCode = http.StatusNotFound
	f := newFakeChat()
	r := chatRunner(t, s, f, runner.OpenCode{Model: "vendor/model"})
	done := runChat(t, r)

	ended := s.waitEnded()
	require.NoError(t, <-done)

	assert.Contains(t, ended[0], "no longer lists this chat")
	assert.Equal(t, 1, f.closed)
}

func TestTheRunnerRefusesAChatItsRulesDoNotAllow(t *testing.T) {
	tests := map[string]struct {
		harness runner.Harness
		change  func(*runner.ClaimedJob)
		want    string
	}{
		"a Mac running Claude Code": {
			runner.ClaudeCode{}, func(*runner.ClaimedJob) {}, "no chat mode",
		},
		"a repo outside the allowlist": {
			runner.OpenCode{Model: "vendor/model"},
			func(j *runner.ClaimedJob) { j.RepoName = "other" }, "allowlist",
		},
		"a branch outside agent/": {
			runner.OpenCode{Model: "vendor/model"},
			func(j *runner.ClaimedJob) { j.Branch = "main" }, "outside",
		},
		"a chat id that is no UUID": {
			runner.OpenCode{Model: "vendor/model"},
			func(j *runner.ClaimedJob) { j.ChatID = "../x" }, "invalid chat id",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newChatServer(t)
			tc.change(s.job)
			f := newFakeChat()
			r := chatRunner(t, s, f, tc.harness)

			done := runChat(t, r)

			ended := s.waitEnded()
			require.NoError(t, <-done)
			assert.Contains(t, ended[0], "Refused by the runner")
			assert.Contains(t, ended[0], tc.want)
			assert.Zero(t, f.closed, "no session was opened")
		})
	}
}

func TestAChatThatCannotStartIsEndedWithTheReason(t *testing.T) {
	s := newChatServer(t)
	f := newFakeChat()
	f.fail = errors.New("no VM image")
	r := chatRunner(t, s, f, runner.OpenCode{Model: "vendor/model"})

	done := runChat(t, r)

	ended := s.waitEnded()
	require.NoError(t, <-done)
	assert.Contains(t, ended[0], "could not start: no VM image")
}
