package watchparty

import (
	"context"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	watchpartyv1 "tools.xdoubleu.com/gen/watchparty/v1"
	"tools.xdoubleu.com/gen/watchparty/v1/watchpartyv1connect"
	"tools.xdoubleu.com/internal/constants"
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
	_ *connect.Request[watchpartyv1.GetRoomRequest],
) (*connect.Response[watchpartyv1.GetRoomResponse], error) {
	user := h.currentUser(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
	return connect.NewResponse(&watchpartyv1.GetRoomResponse{
		Room: protoRoomInfo(roomCode, exists, string(role)),
	}), nil
}

// CreateRoom creates a new room for the current user, or returns existing room.
func (h *roomConnectHandler) CreateRoom(
	ctx context.Context,
	_ *connect.Request[watchpartyv1.CreateRoomRequest],
) (*connect.Response[watchpartyv1.CreateRoomResponse], error) {
	user := h.currentUser(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// If already in a room, return its info
	if inRoom, _, _ := h.app.Services.Room.GetRoomForUser(user.ID); inRoom {
		exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
		return connect.NewResponse(&watchpartyv1.CreateRoomResponse{
			Room: protoRoomInfo(roomCode, exists, string(role)),
		}), nil
	}

	h.app.Services.Room.CreateRoom(ctx, user.ID)

	exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
	return connect.NewResponse(&watchpartyv1.CreateRoomResponse{
		Room: protoRoomInfo(roomCode, exists, string(role)),
	}), nil
}

// JoinRoom adds the current user as a viewer to the specified room.
func (h *roomConnectHandler) JoinRoom(
	ctx context.Context,
	req *connect.Request[watchpartyv1.JoinRoomRequest],
) (*connect.Response[watchpartyv1.JoinRoomResponse], error) {
	user := h.currentUser(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	if req.Msg.RoomCode == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	if !h.app.Services.Room.RoomExists(req.Msg.RoomCode) {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}

	ok := h.app.Services.Room.JoinViewer(ctx, req.Msg.RoomCode, user.ID)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
	return connect.NewResponse(&watchpartyv1.JoinRoomResponse{
		Room: protoRoomInfo(roomCode, exists, string(role)),
	}), nil
}

// LeaveRoom removes the current user from their room.
func (h *roomConnectHandler) LeaveRoom(
	ctx context.Context,
	_ *connect.Request[watchpartyv1.LeaveRoomRequest],
) (*connect.Response[watchpartyv1.LeaveRoomResponse], error) {
	user := h.currentUser(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	exists, roomCode, role := h.app.Services.Room.GetRoomForUser(user.ID)
	if !exists {
		return connect.NewResponse(&watchpartyv1.LeaveRoomResponse{}), nil
	}

	switch role {
	case dtos.Viewer:
		h.app.Services.Room.LeaveViewer(ctx, roomCode)
	case dtos.Presenter:
		h.app.Services.Room.RemoveRoom(ctx, roomCode)
	}

	return connect.NewResponse(&watchpartyv1.LeaveRoomResponse{}), nil
}

// Proto conversion helper.

func protoRoomInfo(roomCode string, inRoom bool, role string) *watchpartyv1.RoomInfo {
	return &watchpartyv1.RoomInfo{
		RoomCode: roomCode,
		InRoom:   inRoom,
		Role:     role,
	}
}
