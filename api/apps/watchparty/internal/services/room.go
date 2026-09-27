package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

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

	const (
		cleanupInterval = 5 * time.Minute
		roomMaxAge      = 12 * time.Hour
	)
	rs.startCleanup(ctx, cleanupInterval, roomMaxAge)

	return rs
}

// Room lookup.

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

// Room creation and removal.

// roomCodeBytes gives 64-bit codes: a code is the only thing standing
// between another user and joining the room.
const roomCodeBytes = 8

func newRoomCode() string {
	b := make([]byte, roomCodeBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (rs *RoomService) CreateRoom(ctx context.Context, presenterID string) string {
	code := newRoomCode()

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

// WebSocket handling.

// JoinPresenter attaches conn as the room's presenter socket; only the
// room's presenter may.
func (rs *RoomService) JoinPresenter(
	ctx context.Context,
	code, userID string,
	conn *websocket.Conn,
) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	room, exists := rs.activeRooms[code]
	if !exists || room.Presenter.ID != userID {
		rs.logger.WarnContext(ctx,
			"Refused presenter join",
			slog.String("code", code),
			slog.Bool("roomExists", exists),
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
	// A connected viewer keeps their seat.
	if room.Viewer.ID != "" && room.Viewer.ID != userID && room.Viewer.WS != nil {
		rs.logger.WarnContext(ctx, "Refused viewer join: seat taken",
			slog.String("code", code))
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

// JoinViewerWS attaches conn as the viewer socket; only the viewer who
// joined through the RPC may.
func (rs *RoomService) JoinViewerWS(
	ctx context.Context,
	code, userID string,
	conn *websocket.Conn,
) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	room, exists := rs.activeRooms[code]
	if !exists || room.Viewer.ID == "" || room.Viewer.ID != userID {
		rs.logger.WarnContext(ctx,
			"Refused viewer join",
			slog.String("code", code),
			slog.Bool("roomExists", exists),
		)
		return false
	}

	room.SetViewerWS(conn)
	rs.logger.InfoContext(ctx, "Viewer WebSocket connected", slog.String("code", code))
	return true
}

// DisconnectViewer handles a closed viewer socket; the seat stays with the
// viewer until they leave through the RPC.
func (rs *RoomService) DisconnectViewer(code string, conn *websocket.Conn) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if room, exists := rs.activeRooms[code]; exists {
		room.ClearViewerWS(conn)
	}
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

// Messaging.

func (rs *RoomService) SendToViewer(
	ctx context.Context,
	code string,
	trackMsg dtos.TrackMessage,
) {
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
		rs.logger.ErrorContext(ctx, "write to viewer failed", slog.Any("err", err))
	}
}

func (rs *RoomService) SendToPresenter(
	ctx context.Context,
	code string,
	trackMsg dtos.TrackMessage,
) {
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
		rs.logger.ErrorContext(ctx, "write to presenter failed", slog.Any("err", err))
	}
}

// Automatic cleanup.

func (rs *RoomService) startCleanup(
	ctx context.Context,
	interval, maxAge time.Duration,
) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			rs.runCleanupTick(ctx, maxAge)
		}
	}()
}

// runCleanupTick recovers panics so one bad tick can't kill the cleanup loop.
func (rs *RoomService) runCleanupTick(ctx context.Context, maxAge time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			rs.logger.ErrorContext(
				ctx,
				"room cleanup panicked",
				slog.Any("panic", r),
			)
		}
	}()

	rs.cleanupOldRooms(ctx, maxAge)
}

func (rs *RoomService) cleanupOldRooms(ctx context.Context, maxAge time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now()
	for code, room := range rs.activeRooms {
		if now.Sub(room.LastActive) > maxAge {
			rs.logger.InfoContext(
				ctx,
				"Removing inactive room",
				slog.String("code", code),
			)

			if room.Viewer.WS != nil {
				room.Viewer.WS.Close(websocket.StatusNormalClosure, "room expired")
			}
			delete(rs.activeRooms, code)
		}
	}
}
