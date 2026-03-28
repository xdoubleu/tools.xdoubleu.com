package services

import (
	"context"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	"tools.xdoubleu.com/apps/watchparty/internal/models"
)

type RoomService struct {
	logger      *slog.Logger
	mu          sync.Mutex
	activeRooms map[string]*models.Room
}

func NewRoomService(ctx context.Context, logger *slog.Logger) *RoomService {
	rs := &RoomService{
		logger:      logger,
		mu:          sync.Mutex{},
		activeRooms: make(map[string]*models.Room),
	}

	// Start automatic cleanup
	//nolint:mnd //time durations
	rs.startCleanup(ctx, 5*time.Minute, 12*time.Hour)

	return rs
}

// ----------------------
// Room Lookup
// ----------------------

func (rs *RoomService) GetRoomForUser(
	userID string,
) (bool, string, dtos.Role) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for c, room := range rs.activeRooms {
		if room.Presenter.ID == userID {
			return true, c, dtos.Presenter
		} else if room.Viewer.ID == userID {
			return true, c, dtos.Viewer
		}
	}

	return false, "", ""
}

func (rs *RoomService) RoomExists(code string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	_, exists := rs.activeRooms[code]
	return exists
}

// ----------------------
// Room Creation & Removal
// ----------------------

func (rs *RoomService) CreateRoom(ctx context.Context, presenterID string) string {
	code := uuid.New().String()[:6]

	rs.mu.Lock()
	defer rs.mu.Unlock()

	room := models.NewRoom(presenterID)
	rs.activeRooms[code] = &room

	rs.logger.InfoContext(ctx, "Created room", slog.String("code", code))
	return code
}

func (rs *RoomService) RemoveRoom(ctx context.Context, code string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if _, exists := rs.activeRooms[code]; !exists {
		rs.logger.WarnContext(ctx,
			"Attempted to remove non-existent room",
			slog.String("code", code),
		)
		return false
	}

	delete(rs.activeRooms, code)
	rs.logger.InfoContext(ctx, "Removed room", slog.String("code", code))
	return true
}

// ----------------------
// WebSocket / Client Handling
// ----------------------

func (rs *RoomService) JoinPresenter(ctx context.Context, code string, conn *websocket.Conn) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	room, exists := rs.activeRooms[code]
	if !exists {
		rs.logger.WarnContext(ctx,
			"Attempted to join non-existent room as presenter",
			slog.String("code", code),
		)
		return false
	}

	room.SetPresenterWS(conn)
	rs.logger.InfoContext(ctx, "Presenter connected", slog.String("code", code))
	return true
}

func (rs *RoomService) JoinViewer(ctx context.Context, code, userID string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	room, exists := rs.activeRooms[code]
	if !exists {
		rs.logger.WarnContext(ctx,
			"Attempted to add viewer to non-existent room",
			slog.String("code", code),
		)
		return false
	}

	room.SetViewer(userID)
	rs.logger.InfoContext(ctx,
		"Viewer added",
		slog.String("code", code),
		slog.String("userID", userID),
	)
	return true
}

func (rs *RoomService) JoinViewerWS(ctx context.Context, code string, conn *websocket.Conn) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	room, exists := rs.activeRooms[code]
	if !exists {
		rs.logger.WarnContext(ctx,
			"Attempted to join non-existent room as viewer",
			slog.String("code", code),
		)
		return false
	}

	room.SetViewerWS(conn)
	rs.logger.InfoContext(ctx, "Viewer WebSocket connected", slog.String("code", code))
	return true
}

func (rs *RoomService) LeaveViewer(ctx context.Context, code string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	room, exists := rs.activeRooms[code]
	if !exists {
		rs.logger.WarnContext(ctx,
			"Attempted to remove viewer from non-existent room",
			slog.String("code", code),
		)
		return
	}

	room.RemoveViewer()
	rs.logger.InfoContext(ctx, "Viewer disconnected", slog.String("code", code))
}

// ----------------------
// Messaging
// ----------------------

func (rs *RoomService) SendToViewer(ctx context.Context, code string, trackMsg dtos.TrackMessage) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	room, exists := rs.activeRooms[code]
	if !exists {
		rs.logger.WarnContext(ctx,
			"Attempted to send message to non-existent viewer",
			slog.String("code", code),
		)
		return
	}

	if err := room.SendToViewer(context.Background(), trackMsg); err != nil {
		log.Printf("Write to viewer failed: %v", err)
	}
}

func (rs *RoomService) SendToPresenter(ctx context.Context, code string, trackMsg dtos.TrackMessage) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	room, exists := rs.activeRooms[code]
	if !exists {
		rs.logger.WarnContext(ctx,
			"Attempted to send message to non-existent presenter",
			slog.String("code", code),
		)
		return
	}

	if err := room.SendToPresenter(context.Background(), trackMsg); err != nil {
		log.Printf("Write to presenter failed: %v", err)
	}
}

// ----------------------
// Automatic Cleanup
// ----------------------

func (rs *RoomService) startCleanup(ctx context.Context, interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			rs.cleanupOldRooms(ctx, maxAge)
		}
	}()
}

func (rs *RoomService) cleanupOldRooms(ctx context.Context, maxAge time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now()
	for code, room := range rs.activeRooms {
		if now.Sub(room.LastActive) > maxAge {
			rs.logger.InfoContext(ctx, "Removing inactive room", slog.String("code", code))

			if room.Viewer.WS != nil {
				room.Viewer.WS.Close(websocket.StatusNormalClosure, "room expired")
			}
			delete(rs.activeRooms, code)
		}
	}
}
