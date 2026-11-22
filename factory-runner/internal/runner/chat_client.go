package runner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	// chatReadLimit is the largest frame the server may send: a person's message.
	chatReadLimit = 1 << 20
	chatDialWait  = 15 * time.Second
	chatWriteWait = 10 * time.Second
)

// Frame kinds of a chat socket, as the server sends them.
const (
	frameStopped = "stopped"
	frameMessage = "user_message"
	frameAnswer  = "user_answer"
	frameState   = "state"
)

// ChatFrame is one message of a chat's runner socket. An event carries its seq; a
// state frame has seq 0 and ends the replay a connection starts with.
type ChatFrame struct {
	Seq     int    `json:"seq"`
	Kind    string `json:"kind"`
	Payload string `json:"payload"`
	State   string `json:"state,omitempty"`
}

// ChatEvent is one thing the harness did, for the transcript.
type ChatEvent struct {
	Kind    string `json:"kind"`
	Payload string `json:"payload"`
}

// ChatUpdate is what the runner sends: events and, when set, the state the session
// moves to (idle, working or asking).
type ChatUpdate struct {
	State  string      `json:"state"`
	Events []ChatEvent `json:"events"`
}

// ChatConn is a chat's runner socket.
type ChatConn interface {
	Read(ctx context.Context) (ChatFrame, error)
	Write(ctx context.Context, u ChatUpdate) error
	Close()
}

// DialError is a refused socket handshake.
type DialError struct{ Status int }

func (e *DialError) Error() string {
	return "the server refused the chat socket: status " + strconv.Itoa(e.Status)
}

// ChatGone reports that the server will not take the runner on this chat's socket
// again: the token is revoked, or the chat is not this runner's.
func ChatGone(err error) bool {
	var d *DialError
	if !errors.As(err, &d) {
		return false
	}
	return d.Status == http.StatusUnauthorized || d.Status == http.StatusForbidden ||
		d.Status == http.StatusNotFound
}

// ChatAPI is the server side of chats beyond what a job has: the socket and the
// close. The real Client has it; a JobAPI without it cannot run chats.
type ChatAPI interface {
	// DialChat opens the chat's socket with the device token; the server replays
	// the person's messages after the seq first.
	DialChat(ctx context.Context, token, chatID string, after int) (ChatConn, error)
	// EndChat closes the chat: the server stops it when it still runs and keeps the
	// report, of what the session left behind.
	EndChat(ctx context.Context, token, chatID, report string) error
}

type endChatRequest struct {
	ChatID string `json:"chatId"`
	Report string `json:"report"`
}

// EndChat implements ChatAPI.
func (c *Client) EndChat(ctx context.Context, token, chatID, report string) error {
	return c.call(ctx, "EndChat", token, pairTimeout,
		endChatRequest{ChatID: chatID, Report: report}, &struct{}{})
}

// DialChat implements ChatAPI.
func (c *Client) DialChat(
	ctx context.Context,
	token, chatID string,
	after int,
) (ChatConn, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("the server address: %w", err)
	}
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	u.Path = strings.TrimRight(u.Path, "/") + "/factory/chats/" + url.PathEscape(chatID) +
		"/runner"
	u.RawQuery = url.Values{"after": {strconv.Itoa(after)}}.Encode()

	dialCtx, cancel := context.WithTimeout(ctx, chatDialWait)
	defer cancel()
	//nolint:exhaustruct // the other options default
	conn, resp, err := websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{
		HTTPClient: c.HTTP,
		HTTPHeader: http.Header{"Authorization": {"Bearer " + token}},
	})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil {
			return nil, &DialError{Status: resp.StatusCode}
		}
		return nil, fmt.Errorf("dial the chat socket: %w", err)
	}
	conn.SetReadLimit(chatReadLimit)
	return &wsConn{conn: conn}, nil
}

type wsConn struct{ conn *websocket.Conn }

func (w *wsConn) Read(ctx context.Context) (ChatFrame, error) {
	var f ChatFrame
	err := wsjson.Read(ctx, w.conn, &f)
	return f, err //nolint:wrapcheck // the caller names it
}

func (w *wsConn) Write(ctx context.Context, u ChatUpdate) error {
	ctx, cancel := context.WithTimeout(ctx, chatWriteWait)
	defer cancel()
	return wsjson.Write(ctx, w.conn, u) //nolint:wrapcheck // the caller names it
}

func (w *wsConn) Close() { _ = w.conn.CloseNow() }
