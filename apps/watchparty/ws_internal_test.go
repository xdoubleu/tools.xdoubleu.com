package watchparty

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
)

// ── isExpectedCloseErr ───────────────────────────────────────────────────────

func TestIsExpectedCloseErrNormalClosure(t *testing.T) {
	//nolint:exhaustruct // Reason field is irrelevant for status-code tests
	err := websocket.CloseError{Code: websocket.StatusNormalClosure}
	assert.True(t, isExpectedCloseErr(err))
}

func TestIsExpectedCloseErrGoingAway(t *testing.T) {
	//nolint:exhaustruct // Reason field is irrelevant for status-code tests
	err := websocket.CloseError{Code: websocket.StatusGoingAway}
	assert.True(t, isExpectedCloseErr(err))
}

func TestIsExpectedCloseErrContextCanceled(t *testing.T) {
	assert.True(t, isExpectedCloseErr(context.Canceled))
}

func TestIsExpectedCloseErrContextDeadlineExceeded(t *testing.T) {
	assert.True(t, isExpectedCloseErr(context.DeadlineExceeded))
}

func TestIsExpectedCloseErrEOF(t *testing.T) {
	assert.False(t, isExpectedCloseErr(io.EOF))
}

func TestIsExpectedCloseErrArbitraryError(t *testing.T) {
	assert.False(t, isExpectedCloseErr(errors.New("network error")))
}

func TestIsExpectedCloseErrInternalError(t *testing.T) {
	//nolint:exhaustruct // Reason field is irrelevant for status-code tests
	err := websocket.CloseError{Code: websocket.StatusInternalError}
	assert.False(t, isExpectedCloseErr(err))
}

// ── pingLoop ─────────────────────────────────────────────────────────────────

func TestPingLoopExitsOnContextCancel(t *testing.T) {
	// Server that accepts the WS and responds to pings automatically
	// (coder/websocket handles pong responses transparently).
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.CloseNow() //nolint:errcheck // cleanup in test server
			conn.CloseRead(r.Context())
			<-r.Context().Done()
		}),
	)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	assert.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck // cleanup in test

	done := make(chan struct{})
	go func() {
		pingLoop(ctx, conn, 50*time.Millisecond, 5*time.Second)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// pingLoop exited as expected
	case <-time.After(time.Second):
		t.Fatal("pingLoop did not exit after context was cancelled")
	}
}

func TestPingLoopExitsOnConnectionClose(t *testing.T) {
	// Server that accepts the WS and holds the connection until the server
	// itself is shut down. srv.Close() tears down the TCP connection, which
	// simulates an abrupt network drop (the same root cause as the reported
	// random-disconnect bug).
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.CloseNow() //nolint:errcheck // cleanup in test server
			<-r.Context().Done()
		}),
	)

	ctx := context.Background()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	assert.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck // cleanup in test

	done := make(chan struct{})
	go func() {
		// Short ping timeout (200 ms) so the test doesn't have to wait 10 s
		// for the dead connection to be detected after the TCP drop.
		pingLoop(ctx, conn, 50*time.Millisecond, 200*time.Millisecond)
		close(done)
	}()

	// Shut down the server — tears down the TCP connection without a clean
	// WebSocket close frame, exactly like a proxy idle-timeout drop.
	srv.Close()

	select {
	case <-done:
		// pingLoop detected the broken connection and exited
	case <-time.After(2 * time.Second):
		t.Fatal("pingLoop did not exit after TCP connection was dropped")
	}
}
