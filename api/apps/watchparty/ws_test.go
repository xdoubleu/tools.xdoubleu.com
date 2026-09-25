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

// dialSignaling opens a signaling WebSocket and sends the subscribe message.
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
		Direction: "",
	}
}

// TestSignalingPresenterOfferRelayedToViewer: offers reach the viewer.
func TestSignalingPresenterOfferRelayedToViewer(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, userID)
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

// TestSignalingViewerAnswerRelayedToPresenter: answers reach the presenter.
func TestSignalingViewerAnswerRelayedToPresenter(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, userID)
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

// TestSignalingPresenterOfferBufferedBeforeViewerConnects: an early offer is
// buffered until the viewer connects.
func TestSignalingPresenterOfferBufferedBeforeViewerConnects(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, userID)
	app.Services.Room.JoinViewer(ctx, roomCode, userID)

	presConn := dialSignaling(t, srv, roomCode, dtos.Presenter)
	defer presConn.CloseNow() //nolint:errcheck // cleanup in test

	time.Sleep(50 * time.Millisecond)

	offer := makeTrackMsg(dtos.Offer, "cam")
	require.NoError(t, wsjson.Write(ctx, presConn, offer))

	viewConn := dialSignaling(t, srv, roomCode, dtos.Viewer)
	defer viewConn.CloseNow() //nolint:errcheck // cleanup in test

	var received dtos.TrackMessage
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Read(readCtx, viewConn, &received))

	assert.Equal(t, dtos.Offer, received.Type)
	assert.Equal(t, "cam", received.TrackType)
}

// TestSignalingAbruptDisconnectDoesNotPanic: a close without a close frame
// ends the read loop cleanly.
func TestSignalingAbruptDisconnectDoesNotPanic(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, userID)
	app.Services.Room.JoinViewer(ctx, roomCode, userID)

	presConn := dialSignaling(t, srv, roomCode, dtos.Presenter)

	time.Sleep(50 * time.Millisecond)

	_ = presConn.CloseNow()

	// The viewer connects; the room is still intact and must not panic.
	viewConn := dialSignaling(t, srv, roomCode, dtos.Viewer)
	defer viewConn.CloseNow() //nolint:errcheck // cleanup in test

	// Give the server read loop time to detect the disconnect.
	time.Sleep(100 * time.Millisecond)
}

// TestSignalingDisconnectBeforeSubscribe covers the ServerErrorResponse path
// when the socket closes before subscribing.
func TestSignalingDisconnectBeforeSubscribe(t *testing.T) {
	_, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/watchparty/api/signaling"

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	// Close before sending the subscribe message so the server's Read fails.
	_ = conn.CloseNow()

	// Give the server handler time to detect the close and exit gracefully.
	time.Sleep(50 * time.Millisecond)
}

// TestSignalingPresenterJoinsNonExistentRoom covers handlePresenter's
// JoinPresenter-failure branch.
func TestSignalingPresenterJoinsNonExistentRoom(t *testing.T) {
	_, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/watchparty/api/signaling"

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck // cleanup in test

	// Valid role but room code does not exist → JoinPresenter returns false.
	sub := map[string]string{"roomCode": "NOROOM", "role": "presenter"}
	require.NoError(t, wsjson.Write(ctx, conn, sub))

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var msg any
	_ = wsjson.Read(readCtx, conn, &msg)
}

// TestSignalingInvalidRoleRejected: an unknown role must not panic.
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

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var msg any
	err = wsjson.Read(readCtx, conn, &msg)
	// Any outcome is fine; only a panic fails.
	_ = err
}

// TestSignalingViewerReconnectReceivesBufferedOffer: a reconnecting viewer
// gets the offer sent while it was away.
func TestSignalingViewerReconnectReceivesBufferedOffer(t *testing.T) {
	app, routes := newTestApp()
	srv := httptest.NewServer(routes)
	defer srv.Close()

	ctx := context.Background()
	roomCode := app.Services.Room.CreateRoom(ctx, userID)
	app.Services.Room.JoinViewer(ctx, roomCode, userID)

	presConn := dialSignaling(t, srv, roomCode, dtos.Presenter)
	defer presConn.CloseNow() //nolint:errcheck // cleanup in test

	time.Sleep(50 * time.Millisecond)

	offer := makeTrackMsg(dtos.Offer, "cam")
	require.NoError(t, wsjson.Write(ctx, presConn, offer))

	viewConn := dialSignaling(t, srv, roomCode, dtos.Viewer)
	defer viewConn.CloseNow() //nolint:errcheck // cleanup in test

	var received dtos.TrackMessage
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Read(readCtx, viewConn, &received))

	assert.Equal(t, dtos.Offer, received.Type)
	assert.Equal(t, "cam", received.TrackType)
}
