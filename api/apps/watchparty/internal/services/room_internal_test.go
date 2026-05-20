package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/watchparty/internal/models"
)

func newBareRoomService() *RoomService {
	return &RoomService{
		logger:      logging.NewNopLogger(),
		mu:          sync.Mutex{},
		activeRooms: make(map[string]*models.Room),
	}
}

func TestCleanupOldRoomsRemovesInactiveRoom(t *testing.T) {
	rs := newBareRoomService()

	room := models.NewRoom("presenter-1")
	room.LastActive = time.Now().Add(-13 * time.Hour)
	rs.activeRooms["STALE"] = &room

	rs.cleanupOldRooms(context.Background(), 12*time.Hour)

	_, exists := rs.activeRooms["STALE"]
	assert.False(t, exists)
}

func TestCleanupOldRoomsKeepsActiveRoom(t *testing.T) {
	rs := newBareRoomService()

	room := models.NewRoom("presenter-1")
	room.LastActive = time.Now()
	rs.activeRooms["ACTIVE"] = &room

	rs.cleanupOldRooms(context.Background(), 12*time.Hour)

	_, exists := rs.activeRooms["ACTIVE"]
	assert.True(t, exists)
}

// TestCleanupOldRoomsClosesViewerWS verifies that when an old room is removed,
// its viewer's WebSocket connection is closed (if present).
// Note: This test uses nil for the WS since we can't easily create real
// WebSocket connections in a unit test without a server. Integration tests
// (room_test.go) cover real WS cleanup scenarios.
func TestCleanupOldRoomsClosesViewerWS(t *testing.T) {
	rs := newBareRoomService()

	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")
	// SetViewer updates LastActive, so set it to stale after that.
	room.LastActive = time.Now().Add(-13 * time.Hour)
	// In a real scenario, room.Viewer.WS would be set to a *websocket.Conn
	// and would be closed. Here we just verify the room is removed.
	rs.activeRooms["STALE"] = &room

	rs.cleanupOldRooms(context.Background(), 12*time.Hour)

	// Room should be removed (and any WS in it would be closed).
	_, exists := rs.activeRooms["STALE"]
	assert.False(t, exists)
}
