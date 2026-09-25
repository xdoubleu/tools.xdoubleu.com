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
	assert.True(t, isExpectedCloseErr(io.EOF))
}

func TestIsExpectedCloseErrUnexpectedEOF(t *testing.T) {
	assert.True(t, isExpectedCloseErr(io.ErrUnexpectedEOF))
}

func TestIsExpectedCloseErrArbitraryError(t *testing.T) {
	assert.False(t, isExpectedCloseErr(errors.New("network error")))
}

func TestIsExpectedCloseErrInternalError(t *testing.T) {
	//nolint:exhaustruct // Reason field is irrelevant for status-code tests
	err := websocket.CloseError{Code: websocket.StatusInternalError}
	assert.False(t, isExpectedCloseErr(err))
}

func TestPingLoopExitsOnContextCancel(t *testing.T) {
	// coder/websocket answers pings transparently.
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
	case <-time.After(time.Second):
		t.Fatal("pingLoop did not exit after context was cancelled")
	}
}

func TestPingLoopExitsOnConnectionClose(t *testing.T) {
	// srv.Close() drops TCP abruptly, simulating a network drop.
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
		// Short ping timeout so detection doesn't take 10 s.
		pingLoop(ctx, conn, 50*time.Millisecond, 200*time.Millisecond)
		close(done)
	}()

	// No close frame, like a proxy idle-timeout drop.
	srv.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pingLoop did not exit after TCP connection was dropped")
	}
}
