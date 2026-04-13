package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/watchparty/internal/services"
)

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
