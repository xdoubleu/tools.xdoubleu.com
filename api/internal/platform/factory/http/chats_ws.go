package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/platform/communication/wstools"
	fconnect "tools.xdoubleu.com/internal/platform/factory/connect"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
)

const (
	bearerPrefix = "Bearer "
	// chatTopicWorkers is 1 so a topic delivers its frames in order.
	chatTopicWorkers    = 1
	chatTopicQueue      = 64
	chatWriteTimeout    = 10 * time.Second
	chatRunnerReadLimit = 4 << 20
	// chatCloseGrace lets a stopped chat's last frames drain before its topics go.
	chatCloseGrace = 5 * time.Second
	// stateKind is the frame that reports where the session stands; it has no seq.
	stateKind = "state"
)

// ChatFrame is one JSON message of a chat socket. An event carries its seq, so a
// client that reconnects asks for the events after the last seq it saw, and drops
// any seq it already has: a push may repeat what a replay just sent. A state
// frame has seq 0.
type ChatFrame struct {
	Seq       int    `json:"seq"`
	Kind      string `json:"kind"`
	Payload   string `json:"payload"`
	CreatedAt string `json:"createdAt"`
	State     string `json:"state,omitempty"`
}

// ChatUpdate is what the runner sends: events the harness produced and, when set,
// the state the session moves to.
type ChatUpdate struct {
	State  string `json:"state"`
	Events []struct {
		Kind    string `json:"kind"`
		Payload string `json:"payload"`
	} `json:"events"`
}

// chatRoom is the two topics of one chat: the browser gets every event and each
// state, the runner only what the person said.
type chatRoom struct {
	mu      sync.Mutex
	browser *wstools.Topic
	runner  *wstools.Topic
	cancel  context.CancelFunc
	// sent is the seq of the newest event pushed to the topics.
	sent    int
	closing bool
}

// ChatWS serves the chat sockets and pushes new events to them, replaying the
// persisted transcript from a cursor to whoever connects.
type ChatWS struct {
	Logger  *slog.Logger
	Chats   *services.ChatService
	Runners *services.RunnerService

	ctx   context.Context
	mu    sync.Mutex
	rooms map[uuid.UUID]*chatRoom
}

var _ fconnect.ChatPublisher = (*ChatWS)(nil)

// NewChatWS is the socket side of chats; its topics live until ctx ends.
func NewChatWS(
	ctx context.Context,
	logger *slog.Logger,
	chats *services.ChatService,
	runners *services.RunnerService,
) *ChatWS {
	return &ChatWS{
		Logger: logger, Chats: chats, Runners: runners, ctx: ctx,
		mu: sync.Mutex{}, rooms: make(map[uuid.UUID]*chatRoom),
	}
}

func isPrompt(kind string) bool {
	return kind == models.ChatEventMessage || kind == models.ChatEventAnswer ||
		kind == models.ChatEventStopped
}

func (h *ChatWS) room(id uuid.UUID, lastSeq int) *chatRoom {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[id]; ok && !r.closing {
		return r
	}
	ctx, cancel := context.WithCancel(h.ctx)
	r := &chatRoom{
		mu: sync.Mutex{}, cancel: cancel, sent: lastSeq, closing: false,
		browser: wstools.NewTopic(
			ctx, h.Logger, id.String(), nil, chatTopicWorkers, chatTopicQueue, nil),
		runner: wstools.NewTopic(
			ctx, h.Logger, id.String()+"/runner", nil, chatTopicWorkers, chatTopicQueue, nil),
	}
	h.rooms[id] = r
	return r
}

// Publish pushes the chat's events since the last push, and its state, to the
// sockets watching it. With none watching it does nothing: whoever connects next
// replays from the database.
func (h *ChatWS) Publish(id uuid.UUID) {
	h.mu.Lock()
	r := h.rooms[id]
	h.mu.Unlock()
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	h.flush(id, r)
}

// flush is Publish with the room locked.
func (h *ChatWS) flush(id uuid.UUID, r *chatRoom) {
	ctx, cancel := context.WithTimeout(h.ctx, chatWriteTimeout)
	defer cancel()
	var session *models.ChatSession
	for {
		var events []models.ChatEvent
		var err error
		session, events, err = h.Chats.Feed(ctx, id, r.sent, services.MaxChatEvents)
		if err != nil {
			h.Logger.WarnContext(ctx, "factory: chat push failed", "chat", id, "error", err)
			return
		}
		for i := range events {
			frame := eventFrame(&events[i])
			r.browser.EnqueueEvent(frame)
			if isPrompt(frame.Kind) {
				r.runner.EnqueueEvent(frame)
			}
			r.sent = events[i].Seq
		}
		if len(events) < services.MaxChatEvents {
			break
		}
	}
	r.browser.EnqueueEvent(stateFrame(session.State))
	if session.State == models.ChatStopped {
		r.closing = true
		time.AfterFunc(chatCloseGrace, func() {
			r.cancel()
			h.mu.Lock()
			if h.rooms[id] == r {
				delete(h.rooms, id)
			}
			h.mu.Unlock()
		})
	}
}

func eventFrame(e *models.ChatEvent) ChatFrame {
	return ChatFrame{
		Seq: e.Seq, Kind: e.Kind, Payload: e.Payload, State: "",
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
	}
}

func stateFrame(state models.ChatState) ChatFrame {
	return ChatFrame{
		Seq: 0, Kind: stateKind, Payload: "", CreatedAt: "", State: string(state),
	}
}

// attach replays the events after the cursor to conn, subscribes it to the topic
// for what follows, and ends the replay with a state frame. The room stays
// locked, so no push falls between.
func (h *ChatWS) attach(
	ctx context.Context,
	conn *websocket.Conn,
	id uuid.UUID,
	lastSeq, after int,
	runner bool,
) error {
	r := h.room(id, lastSeq)
	r.mu.Lock()
	defer r.mu.Unlock()
	topic := r.browser
	if runner {
		topic = r.runner
	}
	var state models.ChatState
	for {
		wctx, cancel := context.WithTimeout(ctx, chatWriteTimeout)
		session, events, err := h.Chats.Feed(wctx, id, after, services.MaxChatEvents)
		if err == nil {
			err = writeReplay(wctx, conn, events, runner)
		}
		cancel()
		if err != nil {
			return err
		}
		state = session.State
		if len(events) < services.MaxChatEvents {
			break
		}
		after = events[len(events)-1].Seq
	}
	if err := topic.Subscribe(conn); err != nil {
		return err
	}
	// The state frame ends the replay: the socket is subscribed by now.
	wctx, cancel := context.WithTimeout(ctx, chatWriteTimeout)
	defer cancel()
	return wsjson.Write(wctx, conn, stateFrame(state))
}

func writeReplay(
	ctx context.Context,
	conn *websocket.Conn,
	events []models.ChatEvent,
	runner bool,
) error {
	for i := range events {
		if runner && !isPrompt(events[i].Kind) {
			continue
		}
		if err := wsjson.Write(ctx, conn, eventFrame(&events[i])); err != nil {
			return err
		}
	}
	return nil
}

func chatID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	return id, err == nil
}

func afterSeq(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("after"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, services.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, database.ErrResourceNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// Browser serves GET /factory/chats/{id}/ws to a signed-in admin: the events
// after ?after=<seq> (all by default), then live ones. It reads nothing the
// browser sends; messages go through the ChatService RPCs.
func (h *ChatWS) Browser(w http.ResponseWriter, r *http.Request) {
	id, ok := chatID(r)
	if !ok {
		http.Error(w, "invalid chat id", http.StatusBadRequest)
		return
	}
	actor, err := fconnect.ActorFrom(r.Context())
	if err != nil {
		http.Error(w, "admin access required", http.StatusForbidden)
		return
	}
	session, err := h.Chats.Get(r.Context(), actor, id)
	if err != nil {
		http.Error(w, "chat unavailable", statusOf(err))
		return
	}
	//nolint:exhaustruct // the default same-origin check is what we want
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		wstools.UpgradeErrorResponse(w, r, err)
		return
	}
	err = h.attach(r.Context(), conn, id, session.LastSeq, afterSeq(r), false)
	if err != nil {
		wstools.ServerErrorResponse(r.Context(), conn, err)
		_ = conn.Close(websocket.StatusInternalError, "replay failed")
		return
	}
	drain(r.Context(), conn)
}

// drain reads until the peer closes, which is how the library answers pings and
// notices a close.
func drain(ctx context.Context, conn *websocket.Conn) {
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			_ = conn.CloseNow()
			return
		}
	}
}

// Runner serves GET /factory/chats/{id}/runner to the runner the chat is pinned
// to, authenticated by its device token (docs/adr-0041). The runner receives the
// person's messages, answers and the stop after ?after=<seq>, and sends
// ChatUpdate messages back.
func (h *ChatWS) Runner(w http.ResponseWriter, r *http.Request) {
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), bearerPrefix)
	if !found {
		http.Error(w, "no device token", http.StatusUnauthorized)
		return
	}
	runner, err := h.Runners.Authenticate(r.Context(), token)
	if err != nil {
		http.Error(w, "unknown device token", http.StatusUnauthorized)
		return
	}
	id, ok := chatID(r)
	if !ok {
		http.Error(w, "invalid chat id", http.StatusBadRequest)
		return
	}
	session, err := h.Chats.ForRunner(r.Context(), runner.ID, id)
	if err != nil {
		http.Error(w, "chat unavailable", statusOf(err))
		return
	}
	//nolint:exhaustruct // the runner sends no Origin, so the default check passes
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		wstools.UpgradeErrorResponse(w, r, err)
		return
	}
	conn.SetReadLimit(chatRunnerReadLimit)
	err = h.attach(r.Context(), conn, id, session.LastSeq, afterSeq(r), true)
	if err != nil {
		wstools.ServerErrorResponse(r.Context(), conn, err)
		_ = conn.Close(websocket.StatusInternalError, "replay failed")
		return
	}
	h.listen(r.Context(), conn, runner.ID, id)
}

// listen records the runner's updates until it disconnects. A rejected update is
// answered with an error frame; the socket stays open unless the chat stopped.
func (h *ChatWS) listen(
	ctx context.Context,
	conn *websocket.Conn,
	runnerID, id uuid.UUID,
) {
	defer func() { _ = conn.CloseNow() }()
	for {
		var update ChatUpdate
		if err := wsjson.Read(ctx, conn, &update); err != nil {
			return
		}
		events := make([]models.ChatEventInput, len(update.Events))
		for i, e := range update.Events {
			events[i] = models.ChatEventInput{Kind: e.Kind, Payload: e.Payload}
		}
		err := h.Chats.RecordEvents(
			ctx, runnerID, id, models.ChatState(update.State), events)
		if err != nil {
			wstools.ErrorResponse(ctx, conn, http.StatusBadRequest, err.Error())
			if errors.Is(err, services.ErrConflict) {
				return
			}
			continue
		}
		h.Publish(id)
	}
}
