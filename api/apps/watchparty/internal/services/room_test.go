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

	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	"tools.xdoubleu.com/apps/watchparty/internal/services"
	"tools.xdoubleu.com/internal/logging"
)

// closedWSConn returns a closed server-side connection whose writes fail.
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
	_ = srvConn.CloseNow()
	return srvConn
}

func trackMsg() dtos.TrackMessage {
	payload, _ := json.Marshal(map[string]string{"type": "offer", "sdp": "v=0"})
	return dtos.TrackMessage{
		Type:      dtos.Offer,
		Payload:   payload,
		TrackType: "cam",
		Direction: "",
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

	rs.LeaveViewer(context.Background(), "XXXXXX")
}

func TestJoinPresenterToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	ok := rs.JoinPresenter(t.Context(), "XXXXXX", "presenter-1", nil)

	assert.False(t, ok)
}

func TestJoinViewerWSToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	ok := rs.JoinViewerWS(t.Context(), "XXXXXX", "viewer-1", nil)

	assert.False(t, ok)
}

func TestSendToViewerToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	rs.SendToViewer(t.Context(), "XXXXXX", trackMsg())
}

func TestSendToPresenterToNonExistentRoom(t *testing.T) {
	rs := newRoomService(t)

	rs.SendToPresenter(t.Context(), "XXXXXX", trackMsg())
}

func TestSendToViewerWriteError(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	rs.JoinViewer(t.Context(), code, "viewer-1")
	rs.JoinViewerWS(t.Context(), code, "viewer-1", closedWSConn(t))

	// A write to a closed connection is logged, not a panic.
	rs.SendToViewer(t.Context(), code, trackMsg())
}

func TestSendToPresenterWriteError(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	rs.JoinPresenter(t.Context(), code, "presenter-1", closedWSConn(t))

	// A write to a closed connection is logged, not a panic.
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

func TestCreateRoom_CodeHas64Bits(t *testing.T) {
	rs := newRoomService(t)
	assert.Len(t, rs.CreateRoom(t.Context(), "user-1"), 16)
}

// Knowing a room's code doesn't let another user take either seat.
func TestJoinPresenter_OtherUserRefused(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	assert.False(t, rs.JoinPresenter(t.Context(), code, "intruder", nil))
}

func TestJoinViewerWS_UnregisteredUserRefused(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	assert.False(t, rs.JoinViewerWS(t.Context(), code, "viewer-1", nil))

	rs.JoinViewer(t.Context(), code, "viewer-1")
	assert.False(t, rs.JoinViewerWS(t.Context(), code, "intruder", nil))
}

func TestJoinViewer_ConnectedSeatNotReplaced(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	require.True(t, rs.JoinViewer(t.Context(), code, "viewer-1"))
	require.True(t, rs.JoinViewerWS(t.Context(), code, "viewer-1", closedWSConn(t)))

	assert.False(t, rs.JoinViewer(t.Context(), code, "intruder"))
	assert.True(t, rs.JoinViewer(t.Context(), code, "viewer-1"))
}

// A dropped viewer socket keeps the seat, so the client's reconnect works.
func TestDisconnectViewer_KeepsSeatForReconnect(t *testing.T) {
	rs := newRoomService(t)
	code := rs.CreateRoom(t.Context(), "presenter-1")
	require.True(t, rs.JoinViewer(t.Context(), code, "viewer-1"))
	conn := closedWSConn(t)
	require.True(t, rs.JoinViewerWS(t.Context(), code, "viewer-1", conn))

	rs.DisconnectViewer(code, conn)
	rs.DisconnectViewer("XXXXXX", conn)

	assert.True(t, rs.JoinViewerWS(t.Context(), code, "viewer-1", closedWSConn(t)))
	exists, _, role := rs.GetRoomForUser("viewer-1")
	assert.True(t, exists)
	assert.Equal(t, dtos.Viewer, role)
}
