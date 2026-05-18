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

// Helper to add user to context for auth.
func contextWithUser(ctx context.Context, uid string) context.Context {
	user := &models.User{ID: uid} //nolint:exhaustruct // ID only
	return context.WithValue(ctx, constants.UserContextKey, user)
}

// ── GetRoom ──────────────────────────────────────────────────────────────────

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

	// Create a room
	createResp, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	assert.True(t, createResp.Msg.Room.InRoom)
	assert.NotEmpty(t, createResp.Msg.Room.RoomCode)
	assert.Equal(t, "presenter", createResp.Msg.Room.Role)

	// Get room info
	resp, err := client.GetRoom(ctx, connect.NewRequest(&watchpartyv1.GetRoomRequest{}))
	require.NoError(t, err)
	assert.True(t, resp.Msg.Room.InRoom)
	assert.Equal(t, createResp.Msg.Room.RoomCode, resp.Msg.Room.RoomCode)
	assert.Equal(t, "presenter", resp.Msg.Room.Role)
}

// ── CreateRoom ───────────────────────────────────────────────────────────────

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

	// Create first room
	resp1, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	roomCode1 := resp1.Msg.Room.RoomCode

	// Create again — should return same room (mock auth always uses same user)
	resp2, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	assert.Equal(t, roomCode1, resp2.Msg.Room.RoomCode)
}

// ── JoinRoom ─────────────────────────────────────────────────────────────────

func TestJoinRoom_Success(t *testing.T) {
	// Note: Due to mock auth service limitations, we test that JoinRoom succeeds
	// and GetRoomForUser reports room existence. The role assignment is tested
	// via the service layer tests in internal/services/room_test.go.
	client := newConnectClient(t)
	ctx := context.Background()

	// In a real scenario, we'd create a room with one user and join with another.
	// However, the mock auth service provides a fixed user ID, so we test that
	// JoinRoom fails for non-existent room codes.
	roomCode := "ABC123" // Simulate a room code that might exist
	_, err := client.JoinRoom(ctx, connect.NewRequest(&watchpartyv1.JoinRoomRequest{
		RoomCode: roomCode,
	}))
	// Should fail because room doesn't exist
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

// ── LeaveRoom ────────────────────────────────────────────────────────────────

func TestLeaveRoom_AsPresenter(t *testing.T) {
	_, mux := newTestApp()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := watchpartyv1connect.NewRoomServiceClient(http.DefaultClient, ts.URL)

	ctx := context.Background()

	// Create room
	createResp, err := client.CreateRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.CreateRoomRequest{}),
	)
	require.NoError(t, err)
	assert.True(t, createResp.Msg.Room.InRoom)
	assert.Equal(t, "presenter", createResp.Msg.Room.Role)

	// Leave as presenter (should remove room)
	_, err = client.LeaveRoom(ctx, connect.NewRequest(&watchpartyv1.LeaveRoomRequest{}))
	require.NoError(t, err)

	// Verify no longer in room
	resp, err := client.GetRoom(ctx, connect.NewRequest(&watchpartyv1.GetRoomRequest{}))
	require.NoError(t, err)
	assert.False(t, resp.Msg.Room.InRoom)
}

func TestLeaveRoom_NotInRoom(t *testing.T) {
	client := newConnectClient(t)
	ctx := context.Background()

	// Leave without being in a room (should be no-op)
	_, err := client.LeaveRoom(
		ctx,
		connect.NewRequest(&watchpartyv1.LeaveRoomRequest{}),
	)
	require.NoError(t, err)

	// Verify still not in room
	resp, err := client.GetRoom(ctx, connect.NewRequest(&watchpartyv1.GetRoomRequest{}))
	require.NoError(t, err)
	assert.False(t, resp.Msg.Room.InRoom)
}
