package wstools_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/communication/wstools"
	"tools.xdoubleu.com/internal/logging"
)

// TestTopicEnqueueEventDeliversAndUnsubscribesOnClose exercises Topic's
// EnqueueEvent (delivering to a live subscriber) and the UnSubscribe path
// that fires automatically once a subscriber's connection is closed.
func TestTopicEnqueueEventDeliversAndUnsubscribesOnClose(t *testing.T) {
	t.Parallel()

	ws := wstools.CreateWebSocketHandler[testSubscribeMsg](
		t.Context(), logging.NewNopLogger(), 1, 10,
	)
	topic, err := ws.AddTopic("exists", []string{"http://localhost"}, nil)
	require.NoError(t, err)

	ts := httptest.NewServer(ws.Handler())
	defer ts.Close()

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dialCancel()

	conn, _, err := websocket.Dial(
		dialCtx,
		"ws"+strings.TrimPrefix(ts.URL, "http"),
		nil,
	)
	require.NoError(t, err)

	require.NoError(
		t,
		wsjson.Write(dialCtx, conn, testSubscribeMsg{TopicName: "exists"}),
	)

	// Subscribe runs in the server's own goroutine, racing this test; a fixed
	// sleep before publishing (matching essentia's own WebSocketTester) is
	// simpler and safer than polling with short-lived read contexts --
	// coder/websocket closes the connection outright once a Read's context
	// expires, so a short-timeout retry loop would kill the connection on
	// its first miss instead of just moving on.
	time.Sleep(200 * time.Millisecond)
	topic.EnqueueEvent(testResponse{Ok: true})

	readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readCancel()
	var got testResponse
	require.NoError(t, wsjson.Read(readCtx, conn, &got))
	assert.True(t, got.Ok)

	require.NoError(t, conn.CloseNow())

	// EnqueueEvent after the client disconnected should not panic; the write
	// failure routes through ServerErrorResponse -> isCloseError -> UnSubscribe.
	assert.NotPanics(t, func() {
		topic.EnqueueEvent(testResponse{Ok: true})
		time.Sleep(50 * time.Millisecond)
	})
}
