package watchparty_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	watchpartyv1 "tools.xdoubleu.com/gen/watchparty/v1"
	"tools.xdoubleu.com/gen/watchparty/v1/watchpartyv1connect"
)

// connectedConn returns a server-side socket for seating a viewer.
func connectedConn(t *testing.T) *websocket.Conn {
	t.Helper()
	connCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			connCh <- conn
			<-r.Context().Done()
		},
	))
	t.Cleanup(srv.Close)
	client, _, err := websocket.Dial(
		t.Context(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.CloseNow() })
	return <-connCh
}

func TestErrorCodes(t *testing.T) {
	app, mux := newTestApp()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := watchpartyv1connect.NewRoomServiceClient(connect.NewClient(
		connecthttp.NewTransport(http.DefaultClient, ts.URL),
	))

	ctx := context.Background()
	takenRoom := app.Services.Room.CreateRoom(ctx, "presenter-2")
	require.NoError(t, app.Services.Room.JoinViewer(ctx, takenRoom, "viewer-2"))
	require.True(t, app.Services.Room.JoinViewerWS(
		ctx, takenRoom, "viewer-2", connectedConn(t),
	))

	tests := []struct {
		name string
		code string
		want connect.Code
	}{
		{"missing room code", "", connect.CodeInvalidArgument},
		{"unknown room", "XXXXXX", connect.CodeNotFound},
		{"viewer seat taken", takenRoom, connect.CodeFailedPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.JoinRoom(
				ctx, &watchpartyv1.JoinRoomRequest{RoomCode: tt.code},
			)
			assert.Equal(t, tt.want, connect.CodeOf(err))
		})
	}
}
