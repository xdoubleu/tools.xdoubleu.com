package services_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	"tools.xdoubleu.com/apps/watchparty/internal/services"
)

// closedWSConn creates a real websocket connection on the server side and
// immediately closes it so that any subsequent write to it returns an error.
func closedWSConn(t *testing.T) *websocket.Conn {
	t.Helper()

	connCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			connCh <- conn
			<-r.Context().Done()
		}),
	)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	clientConn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientConn.CloseNow() })

	srvConn := <-connCh
	// Close the server-side connection so any write attempt returns an error.
	_ = srvConn.CloseNow()
	return srvConn
}

func trackMsg() dtos.TrackMessage {
	payload, _ := json.Marshal(map[string]string{"type": "offer", "sdp": "v=0"})
	return dtos.TrackMessage{
		Type:      dtos.Offer,
		Payload:   payload,
		TrackType: "cam",
	}
}

func newRoomService(t *testing.T) *services.RoomService {
	t.Helper()
	return services.NewRoomService(t.Context(), logging.NewNopLogger())
}

func TestCreateRoom(t *testing.T) {
	rs := newRoomService(t)

	code := rs.CreateRoom(t.Context(), "user-1")

	assert.NotEmpty(t, code)
	assert.True(t, rs.RoomExists(code))
}

func TestCreateRoomCodeIsUnique(t *testing.T) {
	rs := newRoomService(t)

	code1 := rs.CreateRoom(t.Context(), "user-1")
	code2 := rs.CreateRoom(t.Context(), "user-2")

	assert.NotEqual(t, code1, code2)
}

func TestRoomExistsReturnsFalseForUnknown(t *testing.T) {
	rs := newRoomService(t)

	assert.False(t, rs.RoomExists("XXXXXX"))
}

func TestRemoveRoom(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "user-1")

	removed := rs.RemoveRoom(t.Context(), code)

	assert.True(t, removed)
	assert.False(t, rs.RoomExists(code))
}

func TestRemoveNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	removed := rs.RemoveRoom(t.Context(), "XXXXXX")

	assert.False(t, removed)
}

func TestGetRoomForUserAsPresenter(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")

	exists, roomCode, role := rs.GetRoomForUser("presenter-1")

	assert.True(t, exists)
	assert.Equal(t, code, roomCode)
	assert.Equal(t, "presenter", string(role))
}

func TestGetRoomForUserAsViewer(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	rs.JoinViewer(t.Context(), code, "viewer-1")

	exists, roomCode, role := rs.GetRoomForUser("viewer-1")

	assert.True(t, exists)
	assert.Equal(t, code, roomCode)
	assert.Equal(t, "viewer", string(role))
}

func TestGetRoomForUserNotInRoom(t *testing.T) {
	rs := newRoomService(t)

	exists, roomCode, role := rs.GetRoomForUser("nobody")

	assert.False(t, exists)
	assert.Empty(t, roomCode)
	assert.Empty(t, role)
}

func TestJoinViewerToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	ok := rs.JoinViewer(t.Context(), "XXXXXX", "viewer-1")

	assert.False(t, ok)
}

func TestJoinViewerSuccess(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")

	ok := rs.JoinViewer(t.Context(), code, "viewer-1")

	assert.True(t, ok)
	exists, _, role := rs.GetRoomForUser("viewer-1")
	assert.True(t, exists)
	assert.Equal(t, "viewer", string(role))
}

func TestLeaveViewer(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	rs.JoinViewer(t.Context(), code, "viewer-1")

	rs.LeaveViewer(t.Context(), code)

	exists, _, _ := rs.GetRoomForUser("viewer-1")
	assert.False(t, exists)
}

func TestLeaveViewerFromNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	// Should not panic
	rs.LeaveViewer(context.Background(), "XXXXXX")
}

func TestJoinPresenterToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	ok := rs.JoinPresenter(t.Context(), "XXXXXX", nil)

	assert.False(t, ok)
}

func TestJoinViewerWSToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	ok := rs.JoinViewerWS(t.Context(), "XXXXXX", nil)

	assert.False(t, ok)
}

func TestSendToViewerToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	// Should not panic
	rs.SendToViewer(t.Context(), "XXXXXX", trackMsg())
}

func TestSendToPresenterToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	// Should not panic
	rs.SendToPresenter(t.Context(), "XXXXXX", trackMsg())
}

func TestSendToViewerWriteError(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	rs.JoinViewer(t.Context(), code, "viewer-1")
	rs.JoinViewerWS(t.Context(), code, closedWSConn(t))

	// Write to a closed connection — the service must log the error and not panic.
	rs.SendToViewer(t.Context(), code, trackMsg())
}

func TestSendToPresenterWriteError(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	rs.JoinPresenter(t.Context(), code, closedWSConn(t))

	// Write to a closed connection — the service must log the error and not panic.
	rs.SendToPresenter(t.Context(), code, trackMsg())
}

func TestGetRoomForUserAfterPresenterRemovesRoom(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	rs.JoinViewer(t.Context(), code, "viewer-1")

	rs.RemoveRoom(t.Context(), code)

	exists, _, _ := rs.GetRoomForUser("presenter-1")
	assert.False(t, exists)
	exists, _, _ = rs.GetRoomForUser("viewer-1")
	assert.False(t, exists)
}
