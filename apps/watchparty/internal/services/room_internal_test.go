package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
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
