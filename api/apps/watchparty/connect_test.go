package watchparty_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
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
	return watchpartyv1connect.NewRoomServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(
				http.DefaultClient,
				ts.URL,
				connecthttp.WithReadMaxBytes(0),
			),
		),
	)
}

// contextWithUser adds a user to ctx for auth.
func contextWithUser(ctx context.Context, uid string) context.Context {
	user := &models.User{ID: uid} //nolint:exhaustruct // ID only
	return context.WithValue(ctx, constants.UserContextKey, user)
}

func TestGetRoom_NotInRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	resp, err := client.GetRoom(ctx, &watchpartyv1.GetRoomRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Room.InRoom)
	assert.Empty(t, resp.Room.RoomCode)
	assert.Empty(t, resp.Room.Role)
}

func TestGetRoom_InRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	createResp, err := client.CreateRoom(
		ctx,
		&watchpartyv1.CreateRoomRequest{},
	)
	require.NoError(t, err)
	assert.True(t, createResp.Room.InRoom)
	assert.NotEmpty(t, createResp.Room.RoomCode)
	assert.Equal(t, "presenter", createResp.Room.Role)

	resp, err := client.GetRoom(ctx, &watchpartyv1.GetRoomRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Room.InRoom)
	assert.Equal(t, createResp.Room.RoomCode, resp.Room.RoomCode)
	assert.Equal(t, "presenter", resp.Room.Role)
}

func TestCreateRoom_Success(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	resp, err := client.CreateRoom(
		ctx,
		&watchpartyv1.CreateRoomRequest{},
	)
	require.NoError(t, err)
	assert.True(t, resp.Room.InRoom)
	assert.NotEmpty(t, resp.Room.RoomCode)
	assert.Equal(t, "presenter", resp.Room.Role)
}

func TestCreateRoom_AlreadyInRoom(t *testing.T) {
	_, mux := newTestApp()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := watchpartyv1connect.NewRoomServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(
				http.DefaultClient,
				ts.URL,
				connecthttp.WithReadMaxBytes(0),
			),
		),
	)

	ctx := context.Background()

	resp1, err := client.CreateRoom(
		ctx,
		&watchpartyv1.CreateRoomRequest{},
	)
	require.NoError(t, err)
	roomCode1 := resp1.Room.RoomCode

	// Same room again: mock auth always uses the same user.
	resp2, err := client.CreateRoom(
		ctx,
		&watchpartyv1.CreateRoomRequest{},
	)
	require.NoError(t, err)
	assert.Equal(t, roomCode1, resp2.Room.RoomCode)
}

func TestJoinRoom_Success(t *testing.T) {
	// Mock auth has one fixed user, so role assignment is covered in
	// internal/services/room_test.go; this only checks a missing room fails.
	client := newConnectClient(t)
	ctx := context.Background()

	roomCode := "ABC123" // Simulate a room code that might exist
	_, err := client.JoinRoom(ctx, &watchpartyv1.JoinRoomRequest{
		RoomCode: roomCode,
	})
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestJoinRoom_EmptyRoomCode(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	_, err := client.JoinRoom(ctx, &watchpartyv1.JoinRoomRequest{
		RoomCode: "",
	})
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestJoinRoom_NonExistentRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := contextWithUser(context.Background(), userID)

	_, err := client.JoinRoom(ctx, &watchpartyv1.JoinRoomRequest{
		RoomCode: "nonexistent",
	})
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestLeaveRoom_AsPresenter(t *testing.T) {
	_, mux := newTestApp()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := watchpartyv1connect.NewRoomServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(
				http.DefaultClient,
				ts.URL,
				connecthttp.WithReadMaxBytes(0),
			),
		),
	)

	ctx := context.Background()

	createResp, err := client.CreateRoom(
		ctx,
		&watchpartyv1.CreateRoomRequest{},
	)
	require.NoError(t, err)
	assert.True(t, createResp.Room.InRoom)
	assert.Equal(t, "presenter", createResp.Room.Role)

	// Leaving as presenter removes the room.
	_, err = client.LeaveRoom(ctx, &watchpartyv1.LeaveRoomRequest{})
	require.NoError(t, err)

	resp, err := client.GetRoom(ctx, &watchpartyv1.GetRoomRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Room.InRoom)
}

func TestLeaveRoom_NotInRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := context.Background()

	// Leaving without a room is a no-op.
	_, err := client.LeaveRoom(
		ctx,
		&watchpartyv1.LeaveRoomRequest{},
	)
	require.NoError(t, err)

	resp, err := client.GetRoom(ctx, &watchpartyv1.GetRoomRequest{})
	require.NoError(t, err)
	assert.False(t, resp.Room.InRoom)
}
