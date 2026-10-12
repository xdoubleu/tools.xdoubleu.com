package watchparty

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	watchpartyv1 "tools.xdoubleu.com/gen/watchparty/v1"
	"tools.xdoubleu.com/gen/watchparty/v1/watchpartyv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/models"
)

type roomConnectHandler struct{ app *WatchParty }

var _ watchpartyv1connect.RoomServiceHandler = (*roomConnectHandler)(nil)

func (h *roomConnectHandler) currentUser(ctx context.Context) *models.User {
	return contexttools.GetValue[models.User](ctx, constants.UserContextKey)
}

// GetRoom returns the current user's room information.
func (h *roomConnectHandler) GetRoom(
	ctx context.Context,
	_ *watchpartyv1.GetRoomRequest,
) (*watchpartyv1.GetRoomResponse, error) {
	user := h.currentUser(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}

	exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
	return &watchpartyv1.GetRoomResponse{
		Room: protoRoomInfo(roomCode, exists, string(role)),
	}, nil
}

// CreateRoom creates a new room for the current user, or returns existing room.
func (h *roomConnectHandler) CreateRoom(
	ctx context.Context,
	_ *watchpartyv1.CreateRoomRequest,
) (*watchpartyv1.CreateRoomResponse, error) {
	user := h.currentUser(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}

	if inRoom, _, _ := h.app.Services.Room.GetRoomForUser(user.ID); inRoom {
		exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
		return &watchpartyv1.CreateRoomResponse{
			Room: protoRoomInfo(roomCode, exists, string(role)),
		}, nil
	}

	h.app.Services.Room.CreateRoom(ctx, user.ID)

	exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
	return &watchpartyv1.CreateRoomResponse{
		Room: protoRoomInfo(roomCode, exists, string(role)),
	}, nil
}

// JoinRoom adds the current user as a viewer to the specified room.
func (h *roomConnectHandler) JoinRoom(
	ctx context.Context,
	req *watchpartyv1.JoinRoomRequest,
) (*watchpartyv1.JoinRoomResponse, error) {
	user := h.currentUser(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}

	if req.RoomCode == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "")
	}

	if !h.app.Services.Room.RoomExists(req.RoomCode) {
		return nil, connect.NewError(connect.CodeNotFound, "")
	}

	ok := h.app.Services.Room.JoinViewer(ctx, req.RoomCode, user.ID)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, "")
	}

	exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
	return &watchpartyv1.JoinRoomResponse{
		Room: protoRoomInfo(roomCode, exists, string(role)),
	}, nil
}

// LeaveRoom removes the current user from their room.
func (h *roomConnectHandler) LeaveRoom(
	ctx context.Context,
	_ *watchpartyv1.LeaveRoomRequest,
) (*watchpartyv1.LeaveRoomResponse, error) {
	user := h.currentUser(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}

	exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
	if !exists {
		return &watchpartyv1.LeaveRoomResponse{}, nil
	}

	switch role {
	case dtos.Viewer:
		h.app.Services.Room.LeaveViewer(ctx, roomCode)
	case dtos.Presenter:
		h.app.Services.Room.RemoveRoom(ctx, roomCode)
	}

	return &watchpartyv1.LeaveRoomResponse{}, nil
}

func protoRoomInfo(roomCode string, inRoom bool, role string) *watchpartyv1.RoomInfo {
	return &watchpartyv1.RoomInfo{
		RoomCode: roomCode,
		InRoom:   inRoom,
		Role:     role,
	}
}
