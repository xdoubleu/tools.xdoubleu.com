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

	// Subscribe races this test. A read with a short deadline would close the
	// connection (coder/websocket), so sleep instead of polling.
	time.Sleep(200 * time.Millisecond)
	topic.EnqueueEvent(testResponse{Ok: true})

	readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readCancel()
	var got testResponse
	require.NoError(t, wsjson.Read(readCtx, conn, &got))
	assert.True(t, got.Ok)

	require.NoError(t, conn.CloseNow())

	assert.NotPanics(t, func() {
		topic.EnqueueEvent(testResponse{Ok: true})
		time.Sleep(50 * time.Millisecond)
	})
}

func TestNewSubscriberHasID(t *testing.T) {
	t.Parallel()

	topic := wstools.NewTopic(
		t.Context(), logging.NewNopLogger(), "topic", nil, 1, 10, nil,
	)
	sub := wstools.NewSubscriber(topic, nil)
	assert.NotEmpty(t, sub.ID())
}
