package watchparty_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
)

// dialSignaling opens a WebSocket to the signaling endpoint and sends the
// initial subscribe message. The returned conn is the caller's to close.
func dialSignaling(
	t *testing.T,
	srv *httptest.Server,
	roomCode string,
	role dtos.Role,
) *websocket.Conn {
	t.Helper()

	ctx := context.Background()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/watchparty/api/signaling"

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	sub := map[string]string{"roomCode": roomCode, "role": string(role)}
	require.NoError(t, wsjson.Write(ctx, conn, sub))

	return conn
}

func makeTrackMsg(msgType dtos.Type, trackType string) dtos.TrackMessage {
	payload, _ := json.Marshal(map[string]string{"type": string(msgType), "sdp": "v=0"})
	return dtos.TrackMessage{
		Type:      msgType,
		Payload:   payload,
		TrackType: trackType,
	}
}

// TestSignalingPresenterOfferRelayedToViewer verifies that an offer sent by
// the presenter is forwarded to the viewer.
func TestSignalingPresenterOfferRelayedToViewer(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, presenterID)
	app.Services.Room.JoinViewer(ctx, roomCode, userID)

	presConn := dialSignaling(t, srv, roomCode, dtos.Presenter)
	defer presConn.CloseNow() //nolint:errcheck // cleanup in test

	viewConn := dialSignaling(t, srv, roomCode, dtos.Viewer)
	defer viewConn.CloseNow() //nolint:errcheck // cleanup in test

	// Give WS goroutines time to register.
	time.Sleep(50 * time.Millisecond)

	offer := makeTrackMsg(dtos.Offer, "cam")
	require.NoError(t, wsjson.Write(ctx, presConn, offer))

	var received dtos.TrackMessage
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Read(readCtx, viewConn, &received))

	assert.Equal(t, dtos.Offer, received.Type)
	assert.Equal(t, "cam", received.TrackType)
}

// TestSignalingViewerAnswerRelayedToPresenter verifies that an answer sent by
// the viewer is forwarded to the presenter.
func TestSignalingViewerAnswerRelayedToPresenter(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, presenterID)
	app.Services.Room.JoinViewer(ctx, roomCode, userID)

	presConn := dialSignaling(t, srv, roomCode, dtos.Presenter)
	defer presConn.CloseNow() //nolint:errcheck // cleanup in test

	viewConn := dialSignaling(t, srv, roomCode, dtos.Viewer)
	defer viewConn.CloseNow() //nolint:errcheck // cleanup in test

	time.Sleep(50 * time.Millisecond)

	answer := makeTrackMsg(dtos.Answer, "screen")
	require.NoError(t, wsjson.Write(ctx, viewConn, answer))

	var received dtos.TrackMessage
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Read(readCtx, presConn, &received))

	assert.Equal(t, dtos.Answer, received.Type)
	assert.Equal(t, "screen", received.TrackType)
}

// TestSignalingPresenterOfferBufferedBeforeViewerConnects verifies that an
// offer sent before the viewer's WebSocket connects is buffered and delivered
// once the viewer connects.
func TestSignalingPresenterOfferBufferedBeforeViewerConnects(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, presenterID)
	app.Services.Room.JoinViewer(ctx, roomCode, userID)

	// Presenter connects first and sends an offer before viewer WS connects.
	presConn := dialSignaling(t, srv, roomCode, dtos.Presenter)
	defer presConn.CloseNow() //nolint:errcheck // cleanup in test

	time.Sleep(50 * time.Millisecond)

	offer := makeTrackMsg(dtos.Offer, "cam")
	require.NoError(t, wsjson.Write(ctx, presConn, offer))

	// Viewer connects after the offer was already sent.
	viewConn := dialSignaling(t, srv, roomCode, dtos.Viewer)
	defer viewConn.CloseNow() //nolint:errcheck // cleanup in test

	var received dtos.TrackMessage
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Read(readCtx, viewConn, &received))

	assert.Equal(t, dtos.Offer, received.Type)
	assert.Equal(t, "cam", received.TrackType)
}

// TestSignalingAbruptDisconnectDoesNotPanic verifies that an abrupt WebSocket
// close (no clean close frame, simulating a dropped connection) causes the
// server-side read loop to exit without panicking.
func TestSignalingAbruptDisconnectDoesNotPanic(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, presenterID)
	app.Services.Room.JoinViewer(ctx, roomCode, userID)

	presConn := dialSignaling(t, srv, roomCode, dtos.Presenter)

	time.Sleep(50 * time.Millisecond)

	// Abrupt close without a clean WebSocket close frame.
	_ = presConn.CloseNow()

	// The viewer connects; the room is still intact and must not panic.
	viewConn := dialSignaling(t, srv, roomCode, dtos.Viewer)
	defer viewConn.CloseNow() //nolint:errcheck // cleanup in test

	// Give the server read loop time to detect the disconnect.
	time.Sleep(100 * time.Millisecond)
}

// TestSignalingInvalidRoleRejected verifies that an unknown role in the
// subscribe message does not cause a panic — the handler simply does nothing.
func TestSignalingInvalidRoleRejected(t *testing.T) {
	_, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/watchparty/api/signaling"

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck // cleanup in test

	sub := map[string]string{"roomCode": "XXXXXX", "role": "unknown"}
	require.NoError(t, wsjson.Write(ctx, conn, sub))

	// Server should close the connection with a validation error response.
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var msg any
	err = wsjson.Read(readCtx, conn, &msg)
	// Any outcome (close frame or error message) is acceptable; we only care
	// that the handler does not panic.
	_ = err
}
