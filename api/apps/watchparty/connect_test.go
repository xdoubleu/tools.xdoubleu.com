package watchparty_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	watchpartyv1 "tools.xdoubleu.com/gen/watchparty/v1"
	"tools.xdoubleu.com/gen/watchparty/v1/watchpartyv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func newConnectClient(t *testing.T) watchpartyv1connect.RoomServiceClient {
	t.Helper()
	_, mux := newTestApp()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return watchpartyv1connect.NewRoomServiceClient(http.DefaultClient, ts.URL)
}

// contextWithUser adds a user to ctx for auth.
func contextWithUser(ctx context.Context, uid string) context.Context {
	user := &models.User{ID: uid} //nolint:exhaustruct // ID only
	return context.WithValue(ctx, constants.UserContextKey, user)
}

func TestGetRoom_NotInRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	resp, err := client.GetRoom(ctx, connect.NewRequest(&watchpartyv1.GetRoomRequest{}))
	require.NoError(t, err)
	assert.False(t, resp.Msg.Room.InRoom)
	assert.Empty(t, resp.Msg.Room.RoomCode)
	assert.Empty(t, resp.Msg.Room.Role)
}

func TestGetRoom_InRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	createResp, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	assert.True(t, createResp.Msg.Room.InRoom)
	assert.NotEmpty(t, createResp.Msg.Room.RoomCode)
	assert.Equal(t, "presenter", createResp.Msg.Room.Role)

	resp, err := client.GetRoom(ctx, connect.NewRequest(&watchpartyv1.GetRoomRequest{}))
	require.NoError(t, err)
	assert.True(t, resp.Msg.Room.InRoom)
	assert.Equal(t, createResp.Msg.Room.RoomCode, resp.Msg.Room.RoomCode)
	assert.Equal(t, "presenter", resp.Msg.Room.Role)
}

func TestCreateRoom_Success(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	resp, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Room.InRoom)
	assert.NotEmpty(t, resp.Msg.Room.RoomCode)
	assert.Equal(t, "presenter", resp.Msg.Room.Role)
}

func TestCreateRoom_AlreadyInRoom(t *testing.T) {
	_, mux := newTestApp()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := watchpartyv1connect.NewRoomServiceClient(http.DefaultClient, ts.URL)

	ctx := context.Background()

	resp1, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	roomCode1 := resp1.Msg.Room.RoomCode

	// Same room again: mock auth always uses the same user.
	resp2, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	assert.Equal(t, roomCode1, resp2.Msg.Room.RoomCode)
}

func TestJoinRoom_Success(t *testing.T) {
	// Mock auth has one fixed user, so role assignment is covered in
	// internal/services/room_test.go; this only checks a missing room fails.
	client := newConnectClient(t)
	ctx := context.Background()

	roomCode := "ABC123" // Simulate a room code that might exist
	_, err := client.JoinRoom(ctx, connect.NewRequest(&watchpartyv1.JoinRoomRequest{
		RoomCode: roomCode,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestJoinRoom_EmptyRoomCode(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	_, err := client.JoinRoom(ctx, connect.NewRequest(&watchpartyv1.JoinRoomRequest{
		RoomCode: "",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestJoinRoom_NonExistentRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	_, err := client.JoinRoom(ctx, connect.NewRequest(&watchpartyv1.JoinRoomRequest{
		RoomCode: "nonexistent",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestLeaveRoom_AsPresenter(t *testing.T) {
	_, mux := newTestApp()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := watchpartyv1connect.NewRoomServiceClient(http.DefaultClient, ts.URL)

	ctx := context.Background()

	createResp, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	assert.True(t, createResp.Msg.Room.InRoom)
	assert.Equal(t, "presenter", createResp.Msg.Room.Role)

	// Leaving as presenter removes the room.
	_, err = client.LeaveRoom(ctx, connect.NewRequest(&watchpartyv1.LeaveRoomRequest{}))
	require.NoError(t, err)

	resp, err := client.GetRoom(ctx, connect.NewRequest(&watchpartyv1.GetRoomRequest{}))
	require.NoError(t, err)
	assert.False(t, resp.Msg.Room.InRoom)
}

func TestLeaveRoom_NotInRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := context.Background()

	// Leaving without a room is a no-op.
	_, err := client.LeaveRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.LeaveRoomRequest{}),
	)
	require.NoError(t, err)

	resp, err := client.GetRoom(ctx, connect.NewRequest(&watchpartyv1.GetRoomRequest{}))
	require.NoError(t, err)
	assert.False(t, resp.Msg.Room.InRoom)
}
