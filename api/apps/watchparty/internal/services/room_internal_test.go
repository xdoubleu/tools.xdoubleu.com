package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/watchparty/internal/models"
	"tools.xdoubleu.com/internal/logging"
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

// TestCleanupOldRoomsClosesViewerWS: a stale room is removed (WS is nil here;
// room_test.go covers real sockets).
func TestCleanupOldRoomsClosesViewerWS(t *testing.T) {
	rs := newBareRoomService()

	room := models.NewRoom("presenter-1")
	room.SetViewer("viewer-1")
	// SetViewer updates LastActive, so set it to stale after that.
	room.LastActive = time.Now().Add(-13 * time.Hour)
	rs.activeRooms["STALE"] = &room

	rs.cleanupOldRooms(context.Background(), 12*time.Hour)

	_, exists := rs.activeRooms["STALE"]
	assert.False(t, exists)
}
